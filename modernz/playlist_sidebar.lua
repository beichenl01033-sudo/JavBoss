local mp = require "mp"
local options = require "mp.options"
local utils = require "mp.utils"

local opts = {
    enabled = true,
    width = 320,
    min_width = 240,
    max_width_ratio = 0.58,
    resize_handle_width = 10,
    header_height = 58,
    footer_height = 34,
    control_bar_height = 96,
    row_height = 35,
    show_covers = true,
    cover_preview = true,
    dock_sidebar = true,
    preview_width = 240,
    min_preview_width = 180,
    font_size = 18,
    font = "auto",
    scroll_rows = 3,
    auto_hide_single = true,
    hide_fullscreen = true,
    title = "播放列表",
}

options.read_options(opts, "playlist_sidebar")

local function resolve_font()
    if opts.font and opts.font ~= "" and opts.font ~= "auto" then
        return opts.font
    end
    local platform = mp.get_property("platform") or ""
    if platform == "windows" then
        return "Microsoft YaHei UI"
    elseif platform == "darwin" then
        return "PingFang SC"
    end
    return "Noto Sans CJK SC"
end

local sidebar_font = resolve_font():gsub("[\\{}]", "")

local section = "javboss-playlist-sidebar"
local width_property = "user-data/javboss/playlist-sidebar-width"
local overlay = mp.create_osd_overlay("ass-events")
local visible_start = 1
local visible_rows = 0
local row_height = opts.row_height
local last_playlist_pos = -1
local render_pending = false
local sidebar_visible = false
local sidebar_collapsed = false
local current_width = opts.width
local pane_left = 0
local window_width = 0
local window_height = 0
local panel_bottom = 0
local scale_x = 1
local scale_y = 1
local dragging = false
local click_armed = false
local handle_hovered = false
local handle_mouse_y = nil
local controls_pinned = false
local explicitly_opened = false
local rows_dirty = true
local rows, groups, entry_rows = {}, {}, {}
local playlist_snapshot, points_snapshot, visuals = {}, {}, {}
local images, collapsed_groups, bitmap_ids = {}, {}, {}
local generation = ""
local request_serial = 0
local last_cover_request = ""
local hit_rows = {}
local enhanced = false
local content_bottom = 0
local thumbnail_overlay_id = 42
local last_active_group = nil
local preview_open = opts.cover_preview
local preview_width = 0
local list_right = 0
local preview_hover_id = nil
local refresh_hover = false
local base_margin_right = mp.get_property_number("video-margin-ratio-right", 0) or 0
local dock_serial, dock_signature = 0, ""
local dock_waiting = false
local request_render

local function request_window_dock(preferred)
    if dragging then return end -- Commit a dragged width on release, not mid-gesture.
    local w, h = mp.get_osd_size()
    if not w or not h or w < 1 or h < 1 then return end
    if not opts.dock_sidebar then preferred = 0 end
    local fullscreen = mp.get_property_native("fullscreen", false)
    local maximized = mp.get_property_native("window-maximized", false)
    local hidpi = mp.get_property_native("hidpi-window-scale", false)
    local signature = table.concat({w,h,preferred,tostring(fullscreen),tostring(maximized),tostring(hidpi)}, ":")
    if signature == dock_signature then return end
    dock_signature, dock_serial = signature, dock_serial+1
    dock_waiting = true
    local serial = dock_serial
    mp.add_timeout(0.75, function()
        if dock_serial == serial then dock_waiting=false; request_render() end
    end)
    mp.set_property_native("user-data/javboss/window-dock-request", {serial=dock_serial,
        width=w,height=h,preferred=preferred,fullscreen=fullscreen,maximized=maximized,hidpi=hidpi})
end

local function set_video_margin(ratio)
    if opts.dock_sidebar and dock_waiting then
        local layout = mp.get_property_native("user-data/javboss/window-dock-layout", {}) or {}
        local w = select(1, mp.get_osd_size()) or 0
        local acknowledged = layout.serial == dock_serial
        local arrived = type(layout.window_width) == "number" and math.abs(layout.window_width-w) <= 1
        if not arrived or (not acknowledged and not layout.expanded) then return end
    end
    local margin = math.min(0.95, base_margin_right + ratio)
    if math.abs((mp.get_property_number("video-margin-ratio-right", 0) or 0)-margin) > 0.0001 then
        mp.set_property_number("video-margin-ratio-right", margin)
    end
end

local function clear_bitmaps()
    for id in pairs(bitmap_ids) do mp.commandv("overlay-remove", id) end
    bitmap_ids = {}
end

local function clamp(value, minimum, maximum)
    return math.max(minimum, math.min(maximum, value))
end

local function get_mouse_position()
    local x, y = mp.get_mouse_pos()
    return x and math.floor(x / scale_x), y and math.floor(y / scale_y)
end

local function ass_escape(value)
    local text = tostring(value or "")
    text = text:gsub("\\", "\\\\")
    text = text:gsub("{", "\\{")
    text = text:gsub("}", "\\}")
    text = text:gsub("[\r\n]+", " ")
    return text
end

local function entry_label(entry)
    local titles = mp.get_property_native("user-data/javboss/playlist-titles", {})
    local label = titles[tostring(entry.id)] or entry.title
    if not label or label == "" then
        label = entry.filename or ""
        if not label:find("://", 1, true) then
            local _, filename = utils.split_path(label)
            if filename and filename ~= "" then
                label = filename
            end
        end
    end
    return ass_escape(label)
end

local function format_time(seconds)
    local total = math.floor(math.max(0, tonumber(seconds) or 0))
    if total >= 3600 then return string.format("%d:%02d:%02d", math.floor(total / 3600), math.floor(total % 3600 / 60), total % 60) end
    return string.format("%d:%02d", math.floor(total / 60), total % 60)
end

local function publish_width(value)
    local current = mp.get_property_number(width_property, 0) or 0
    if math.abs(current - value) > 0.5 then
        mp.set_property_number(width_property, value)
    end
end

local function pin_controls(value)
    if controls_pinned == value then return end
    controls_pinned = value
    mp.commandv("script-message-to", "modernz", "osc-visibility", value and "always" or "auto", "yes")
end

local function hide_sidebar()
    clear_bitmaps()
    mp.set_property_number("user-data/javboss/playlist-cover-count", 0)
    mp.set_property_number("user-data/javboss/playlist-preview-index", -1)
    mp.set_property_native("user-data/javboss/playlist-preview-visible", false)
    last_cover_request = ""
    if sidebar_visible then
        overlay:remove()
        mp.disable_key_bindings(section)
        mp.set_mouse_area(0, 0, 0, 0, section)
        sidebar_visible = false
    end
    handle_hovered = false
    handle_mouse_y = nil
    preview_hover_id = nil
    publish_width(0)
    request_window_dock(0)
    local layout = mp.get_property_native("user-data/javboss/window-dock-layout", {}) or {}
    local w = select(1, mp.get_osd_size()) or 0
    local owned = opts.dock_sidebar and layout.expanded and type(layout.extra_width) == "number"
        and type(layout.window_width) == "number" and math.abs(layout.window_width-w) <= 3
    set_video_margin(owned and w > 0 and layout.extra_width/w or 0)
    pin_controls(false)
end

-- Visual rows never change the native playlist. Only consecutive points from
-- the same video/file share a header, so reordered queues retain their order.
local function refresh_rows()
    playlist_snapshot = mp.get_property_native("playlist", {}) or {}
    points_snapshot = mp.get_property_native("user-data/javboss/playlist-highlights", {}) or {}
    visuals = mp.get_property_native("user-data/javboss/playlist-visuals", {}) or {}
    local next_generation = tostring(visuals.generation or "")
    if next_generation ~= generation then
        clear_bitmaps()
        generation = next_generation
        images, collapsed_groups = {}, {}
        last_cover_request, request_serial = "", 0
        visible_start, last_playlist_pos = 1, -1
        last_active_group = nil
        preview_hover_id = nil
    end
    enhanced = generation ~= "" and type(visuals.entries) == "table"
    rows, groups, entry_rows = {}, {}, {}
    local index = 1
    local point_playlist = next(points_snapshot) ~= nil
    while index <= #playlist_snapshot do
        local entry = playlist_snapshot[index]
        local point = points_snapshot[tostring(entry.id)]
        local visual = (visuals.entries or {})[tostring(entry.id)] or {}
        if enhanced and point then
            local last = index
            while last < #playlist_snapshot do
                local candidate = playlist_snapshot[last + 1]
                local next_point = points_snapshot[tostring(candidate.id)]
                local next_visual = visuals.entries[tostring(candidate.id)] or {}
                if not next_point or next_point.video_id ~= point.video_id or next_visual.key ~= visual.key then break end
                last = last + 1
            end
            local group = {key=tostring(entry.id), first=index, last=last, cover=visual.key,
                title=visual.title or point.video_title or entry_label(entry), number=#groups+1}
            groups[#groups+1] = group
            local header = {kind="group", index=index, height=64, group=group}
            rows[#rows+1] = header
            group.row = #rows
            for item = index, last do
                entry_rows[item] = group.row
                if not collapsed_groups[group.key] then
                    rows[#rows+1] = {kind="point", index=item, height=32, group=group}
                    entry_rows[item] = #rows
                end
            end
            index = last + 1
        else
            rows[#rows+1] = {kind="video", index=index,
                height=enhanced and opts.show_covers and 64 or (point_playlist and math.max(56, opts.row_height) or opts.row_height),
                cover=visual.key, title=visual.title}
            entry_rows[index] = #rows
            index = index + 1
        end
    end
    rows_dirty = false
end

local function playlist_position(playlist)
    local property_pos = mp.get_property_number("playlist-pos", -1) or -1
    if property_pos >= 0 then
        return property_pos + 1
    end
    for index, entry in ipairs(playlist) do
        if entry.playing or entry.current then
            return index
        end
    end
    return 1
end

local function panel_width(window_width, height)
    local maximum = math.max(1, math.floor(window_width * opts.max_width_ratio))
    local desired_preview = preview_open and enhanced and opts.show_covers and height >= 400 and opts.preview_width or 0
    request_window_dock((current_width+desired_preview)*scale_x)
    local layout = mp.get_property_native("user-data/javboss/window-dock-layout", {}) or {}
    if opts.dock_sidebar and layout.expanded and type(layout.window_width) == "number"
        and math.abs(layout.window_width-window_width*scale_x) <= 3 then
        maximum = math.max(1, window_width-160)
    end
    preview_width = 0
    if preview_open and enhanced and opts.show_covers and height >= 400
        and maximum >= opts.min_width + opts.min_preview_width then
        preview_width = math.min(opts.preview_width, maximum - opts.min_width)
    end
    return math.max(1, math.min(current_width, maximum - preview_width))
end

local function update_interaction_area(width, height, expanded)
    local pixel_width = math.floor(width * scale_x + 0.5)
    local pixel_bottom = math.floor(panel_bottom * scale_y + 0.5)
    if expanded then
        mp.set_mouse_area(0, 0, pixel_width, pixel_bottom, section)
    else
        local left = math.max(0, pane_left - math.floor(opts.resize_handle_width / 2))
        mp.set_mouse_area(math.floor(left * scale_x), 0, pixel_width, pixel_bottom, section)
    end
end

local function append_rect(parts, x1, y1, x2, y2, color, alpha)
    parts[#parts + 1] = string.format(
        "{\\an7\\pos(0,0)\\bord0\\shad0\\1c&H%s&\\1a&H%s&\\p1}m %d %d l %d %d %d %d %d %d{\\p0}",
        color,
        alpha,
        x1,
        y1,
        x2,
        y1,
        x2,
        y2,
        x1,
        y2
    )
end

local function ensure_current_visible(target, available)
    visible_start = clamp(visible_start, 1, math.max(1, #rows))
    if target < visible_start then visible_start = target end
    local height = 0
    for index = visible_start, target do height = height + rows[index].height end
    while visible_start < target and height > available do
        height = height - rows[visible_start].height
        visible_start = visible_start + 1
    end
end

local function draw_cover(parts, key, x, y, slot, requests, box_width, box_height)
    box_width, box_height = box_width or 48, box_height or 48
    append_rect(parts, x, y, x+box_width, y+box_height, "30343D", "00")
    local cover = key and images[key]
    if cover and opts.show_covers then
        -- ASS IDs are a separate namespace; do not reuse thumbfast's bitmap ID.
        local id = slot - 1
        if id >= thumbnail_overlay_id then id = id + 1 end
        if id <= 63 then
            local fit = math.min(box_width / cover.width, box_height / cover.height)
            local draw_width, draw_height = cover.width * fit, cover.height * fit
            local ok, _, err = pcall(mp.command_native, {name="overlay-add", id=id,
                x=math.floor((x+(box_width-draw_width)/2)*scale_x+0.5),
                y=math.floor((y+(box_height-draw_height)/2)*scale_y+0.5), file=cover.path,
                offset=0, fmt="bgra", w=cover.width, h=cover.height, stride=cover.width*4,
                dw=math.max(1,math.floor(draw_width*scale_x+0.5)), dh=math.max(1,math.floor(draw_height*scale_y+0.5))})
            if ok and not err then bitmap_ids[id] = true; return end
            images[key] = false
        end
    elseif key and images[key] == nil then
        requests[#requests+1] = key
    end
    parts[#parts+1] = string.format("{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs%d\\fn%s\\1c&H9297A2&}%s",
        math.floor(x+box_width/2), math.floor(y+box_height/2), box_width > 100 and 16 or 10, sidebar_font,
        images[key] == false and "无封面" or "封面")
end

local function update_preview_hover()
    local x, y = get_mouse_position()
    if not x or not y or x < pane_left or x > window_width or y > panel_bottom then
        preview_hover_id = nil
    elseif x < list_right and y >= opts.header_height then
        for _, hit in ipairs(hit_rows) do
            if y >= hit.top and y < hit.bottom then
                preview_hover_id = playlist_snapshot[hit.row.index].id
                break
            end
        end
    end
end

local function preview_title_lines(title, units)
    local lines, line, used = {}, "", 0
    for char in tostring(title):gmatch("[%z\1-\127\194-\244][\128-\191]*") do
        if char == "\n" or char == "\r" then char = " " end
        local cost = #char > 1 and 1 or 0.55
        if used + cost > units then
            if #lines == 1 then return {lines[1], line .. "…"} end
            lines[#lines+1], line, used = line, "", 0
        end
        line, used = line .. char, used + cost
    end
    lines[#lines+1] = line
    return lines
end

local function draw_preview(parts, index, current, slot, requests)
    local entry = playlist_snapshot[index]
    local visual = (visuals.entries or {})[tostring(entry.id)] or {}
    local point = points_snapshot[tostring(entry.id)]
    local title = visual.title or (point and point.video_title) or entry.title or entry.filename or ""
    append_rect(parts, list_right, opts.header_height, window_width, content_bottom, "11141A", "00")
    append_rect(parts, list_right, opts.header_height, list_right+1, content_bottom, "424650", "00")
    local x, y = list_right+14, opts.header_height+36
    local box_width = preview_width-28
    local box_height = math.min(360, math.max(48, content_bottom-y-88))
    parts[#parts+1] = string.format("{\\an7\\pos(%d,%d)\\bord0\\shad0\\fs12\\fn%s\\1c&H9297A2&}%s",
        x, opts.header_height+12, sidebar_font, index == current and "正在播放" or "封面预览 · 点击列表播放")
    if visual.key then table.insert(requests, 1, visual.key) end
    draw_cover(parts, visual.key, x, y, slot, requests, box_width, box_height)
    for line, text in ipairs(preview_title_lines(title, math.max(4, math.floor(box_width / 16)-1))) do
        parts[#parts+1] = string.format("{\\an7\\pos(%d,%d)\\clip(%d,%d,%d,%d)\\bord0\\shad0\\fs16\\fn%s\\1c&HFFFFFF&}%s",
            x, y+box_height+12+(line-1)*21, x, y+box_height, window_width-10, content_bottom, sidebar_font, ass_escape(text))
    end
    local subtitle = point and string.format("★ %s · %s", format_time(point.time), point.label or "亮点") or string.format("视频 %d / %d", index, #playlist_snapshot)
    parts[#parts+1] = string.format("{\\an7\\pos(%d,%d)\\clip(%d,%d,%d,%d)\\bord0\\shad0\\fs12\\fn%s\\1c&HF0A34A&}%s",
        x, y+box_height+59, x, y+box_height, window_width-10, content_bottom, sidebar_font, ass_escape(subtitle))
end

local function request_covers(keys)
    local unique, seen = {}, {}
    for _, key in ipairs(keys) do
        if not seen[key] and #unique < 63 then unique[#unique+1] = key; seen[key] = true end
    end
    local signature = generation .. ":" .. table.concat(unique, ",")
    if #unique == 0 or signature == last_cover_request then return end
    last_cover_request = signature
    request_serial = request_serial + 1
    mp.set_property_native("user-data/javboss/playlist-cover-request",
        {generation=generation, serial=request_serial, keys=unique})
end

local function render()
    render_pending = false
    if not opts.enabled or sidebar_collapsed then
        hide_sidebar()
        return
    end

    if rows_dirty then refresh_rows() end
    local playlist = playlist_snapshot
    local count = #playlist
    local points = points_snapshot
    local point_playlist = next(points) ~= nil
    row_height = point_playlist and math.max(56, opts.row_height) or opts.row_height
    local fullscreen = mp.get_property_native("fullscreen", false)
    local pixel_width, pixel_height = mp.get_osd_size()
    if not pixel_width or not pixel_height or pixel_width < 1 or pixel_height < 1
        or count == 0
        or (opts.auto_hide_single and count <= 1 and not point_playlist and not explicitly_opened)
        or (opts.hide_fullscreen and fullscreen and not explicitly_opened) then
        hide_sidebar()
        return
    end

    local dpi_scale = mp.get_property_number("display-hidpi-scale", 1)
    if not dpi_scale or dpi_scale ~= dpi_scale or dpi_scale <= 0 or dpi_scale == math.huge then
        dpi_scale = 1
    end
    -- ASS uses logical pixels; mpv mouse areas and the ModernZ width use render pixels.
    -- Derive each axis from the rounded canvas to keep fractional DPI scales aligned.
    local width = math.max(1, math.floor(pixel_width / dpi_scale + 0.5))
    local height = math.max(1, math.floor(pixel_height / dpi_scale + 0.5))
    local next_scale_x, next_scale_y = pixel_width / width, pixel_height / height
    if width ~= window_width or height ~= window_height or next_scale_x ~= scale_x or next_scale_y ~= scale_y then
        dragging = false
        click_armed = false
        handle_hovered = false
        handle_mouse_y = nil
    end
    scale_x, scale_y = next_scale_x, next_scale_y
    local list_width = panel_width(width, height)
    local pane_width = list_width + preview_width
    pane_left = width - pane_width
    list_right = width - preview_width
    window_width = width
    window_height = height
    panel_bottom = math.max(
        opts.header_height + opts.footer_height + (enhanced and 64 or row_height),
        height - opts.control_bar_height
    )
    content_bottom = panel_bottom - opts.footer_height
    local available = content_bottom - opts.header_height
    local current = playlist_position(playlist)
    if current ~= last_playlist_pos then
        preview_hover_id = nil
        local active_group = nil
        for _, group in ipairs(groups) do
            if current >= group.first and current <= group.last then
                active_group = group.key
                if active_group ~= last_active_group and collapsed_groups[group.key] then
                    collapsed_groups[group.key] = nil
                    refresh_rows()
                end
                break
            end
        end
        last_active_group = active_group
    end
    if current ~= last_playlist_pos or width ~= overlay.res_x or height ~= overlay.res_y then
        ensure_current_visible(entry_rows[current] or 1, available)
        last_playlist_pos = current
    else
        visible_start = clamp(visible_start, 1, math.max(1, #rows))
    end

    local reserved_width = pane_width
    local dock_layout = mp.get_property_native("user-data/javboss/window-dock-layout", {}) or {}
    if opts.dock_sidebar and dock_layout.expanded and type(dock_layout.extra_width) == "number"
        and type(dock_layout.window_width) == "number" and math.abs(dock_layout.window_width-width*scale_x) <= 3 then
        reserved_width = math.max(reserved_width, dock_layout.extra_width/scale_x)
    end
    publish_width(reserved_width * scale_x)
    set_video_margin(opts.dock_sidebar and reserved_width/width or 0)
    pin_controls(true)

    clear_bitmaps()
    local parts, requests = {}, {}
    append_rect(parts, pane_left, 0, width, panel_bottom, "17191F", "08")
    append_rect(parts, pane_left, opts.header_height - 1, width, opts.header_height, "424650", "00")
    parts[#parts + 1] = string.format(
        "{\\an4\\pos(%d,%d)\\bord0\\shad0\\fs22\\b1\\fsp0\\fn%s\\1c&HFFFFFF&}%s",
        pane_left + 18,
        math.floor(opts.header_height / 2),
        sidebar_font,
        ass_escape(pane_width < 300 and "列表" or (point_playlist and "亮点播放列表" or opts.title))
    )
    parts[#parts + 1] = string.format(
        "{\\an6\\pos(%d,%d)\\bord0\\shad0\\fs12\\b0\\fsp0\\fn%s\\1c&HAAAEB8&}%d / %d",
        width - (enhanced and opts.show_covers and 110 or 54),
        math.floor(opts.header_height / 2),
        sidebar_font,
        current,
        count
    )
    if enhanced and opts.show_covers then
        append_rect(parts, width-91, 15, width-43, 43, preview_width > 0 and "49331E" or "30343D", "00")
        parts[#parts+1] = string.format("{\\an5\\pos(%d,29)\\bord0\\shad0\\fs12\\fn%s\\1c&H%s&}封面",
            width-67, sidebar_font, preview_width > 0 and "F0A34A" or "CDD0D7")
    end
    parts[#parts + 1] = string.format(
        "{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs24\\b0\\fn%s\\1c&HCDD0D7&}×",
        width - 22,
        math.floor(opts.header_height / 2),
        sidebar_font
    )

    local top = opts.header_height
    hit_rows, visible_rows = {}, 0
    local cover_slot = 0
    for row_index = visible_start, #rows do
        local row = rows[row_index]
        if top + row.height > content_bottom then break end
        visible_rows = visible_rows + 1
        hit_rows[#hit_rows+1] = {top=top, bottom=top+row.height, row=row}
        local index = row.index
        local entry = playlist[index]
        local point = points[tostring(entry.id)]
        local text_y = top + math.floor(row.height / 2)
        local is_current = index == current or (row.kind == "group" and current >= row.group.first and current <= row.group.last)
        if is_current then
            append_rect(parts, pane_left + 8, top + 2, list_right - 8, top + row.height - 2, "49331E", "00")
            append_rect(parts, pane_left + 8, top + 2, pane_left + 12, top + row.height - 2, "F0A34A", "00")
        elseif entry.id == preview_hover_id and preview_width > 0 then
            append_rect(parts, pane_left + 8, top + 2, list_right - 8, top + row.height - 2, "383029", "00")
        elseif visible_rows % 2 == 0 then
            append_rect(parts, pane_left + 8, top + 2, list_right - 8, top + row.height - 2, "20232A", "30")
        end
        if row.kind == "group" or (enhanced and opts.show_covers and row.kind == "video") then
            local group = row.group
            local text_left = pane_left + 18
            if opts.show_covers then
                cover_slot = cover_slot + 1
                draw_cover(parts, group and group.cover or row.cover, pane_left+18, top+8, cover_slot, requests)
                text_left = pane_left + 78
            end
            local title = group and group.title or row.title or entry.title or entry.filename or ""
            parts[#parts+1] = string.format("{\\an4\\pos(%d,%d)\\clip(%d,%d,%d,%d)\\bord0\\shad0\\fs16\\b%d\\fn%s\\1c&H%s&}%s",
                text_left, top+23, text_left, top, list_right-42, top+row.height,
                is_current and 1 or 0, sidebar_font, is_current and "FFFFFF" or "CDD0D7", ass_escape(title))
            local subtitle = group and string.format("%d 个亮点", group.last-group.first+1) or string.format("视频 %d / %d", index, count)
            parts[#parts+1] = string.format("{\\an4\\pos(%d,%d)\\bord0\\shad0\\fs12\\fn%s\\1c&H9297A2&}%s", text_left, top+45, sidebar_font, subtitle)
            if group then
                parts[#parts+1] = string.format("{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs18\\fn%s\\1c&HCDD0D7&}%s", list_right-25, text_y, sidebar_font, collapsed_groups[group.key] and "▸" or "▾")
            end
        elseif row.kind == "point" then
            local label = tostring(point.label or "")
            if label == "" then label = "亮点 " .. tostring(point.number or "") end
            parts[#parts+1] = string.format("{\\an4\\pos(%d,%d)\\clip(%d,%d,%d,%d)\\bord0\\shad0\\fs14\\fn%s\\1c&H%s&}%s %s · %s",
                pane_left+30, text_y, pane_left+18, top, list_right-14, top+row.height, sidebar_font,
                is_current and "F0A34A" or "CDD0D7", is_current and "▶" or "★", format_time(point.time), ass_escape(label))
        else
            parts[#parts + 1] = string.format(
                "{\\an4\\pos(%d,%d)\\bord0\\shad0\\fs15\\b0\\fsp0\\fn%s\\1c&H%s&}%02d",
                pane_left + 18,
                point and (top + 17) or text_y,
                sidebar_font,
                is_current and "F0A34A" or "858A94",
                index
            )
            parts[#parts + 1] = string.format(
                "{\\an4\\pos(%d,%d)\\clip(%d,%d,%d,%d)\\bord0\\shad0\\fs%d\\b%d\\fsp0\\fn%s\\1c&H%s&}%s",
                pane_left + 52,
                point and (top + 17) or text_y,
                pane_left + 14,
                top,
                list_right - 12,
                top + row.height,
                point and 16 or opts.font_size,
                is_current and 1 or 0,
                sidebar_font,
                is_current and "FFFFFF" or "CDD0D7",
                point and ass_escape(point.video_title) or entry_label(entry)
            )
            if point then
                local label = tostring(point.label or "")
                if label == "" then label = "亮点 " .. tostring(point.number or "") end
                parts[#parts + 1] = string.format(
                    "{\\an4\\pos(%d,%d)\\clip(%d,%d,%d,%d)\\bord0\\shad0\\fs13\\fn%s\\1c&HF0A34A&}★ %s · %s",
                    pane_left + 52, top + 39, pane_left + 14, top, list_right - 12, top + row.height,
                    sidebar_font, format_time(point.time), ass_escape(label))
            end
        end
        top = top + row.height
    end
    if refresh_hover then update_preview_hover(); refresh_hover = false end
    local preview_index = current
    if preview_hover_id then
        for index, entry in ipairs(playlist) do
            if entry.id == preview_hover_id then preview_index = index; break end
        end
    end
    if preview_width > 0 then draw_preview(parts, preview_index, current, cover_slot+1, requests) end
    mp.set_property_number("user-data/javboss/playlist-preview-index", preview_width > 0 and preview_index-1 or -1)
    mp.set_property_native("user-data/javboss/playlist-preview-visible", preview_width > 0)
    request_covers(requests)
    local cover_count = 0
    for _ in pairs(bitmap_ids) do cover_count = cover_count + 1 end
    mp.set_property_number("user-data/javboss/playlist-cover-count", cover_count)

    append_rect(parts, pane_left, panel_bottom - opts.footer_height, width, panel_bottom, "111319", "10")
    local handle_active = dragging or handle_hovered
    append_rect(
        parts,
        pane_left,
        0,
        pane_left + (handle_active and 4 or 2),
        panel_bottom,
        handle_active and "F0A34A" or "555A65",
        "00"
    )
    if handle_active then
        local handle_y = clamp(handle_mouse_y or math.floor(panel_bottom / 2), 24, panel_bottom - 24)
        append_rect(parts, pane_left - 13, handle_y - 20, pane_left + 15, handle_y + 20, "272A32", "00")
        parts[#parts + 1] = string.format(
            "{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs23\\b1\\fn%s\\1c&HF0A34A&}↔",
            pane_left + 1,
            handle_y,
            sidebar_font
        )
    end
    parts[#parts + 1] = string.format(
        "{\\an5\\pos(%d,%d)\\clip(%d,%d,%d,%d)\\bord0\\shad0\\fs14\\b0\\fsp0\\fn%s\\1c&H9297A2&}拖动左边缘调宽 · 右上角 × 或列表按钮收起 · 点击播放",
        pane_left + math.floor(pane_width / 2),
        panel_bottom - math.floor(opts.footer_height / 2),
        pane_left + 6,
        panel_bottom - opts.footer_height,
        width - 6,
        panel_bottom,
        sidebar_font
    )
    if preview_open and enhanced and opts.show_covers and preview_width == 0 then
        parts[#parts] = string.format("{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs12\\fn%s\\1c&H9297A2&}加宽窗口可显示大封面 · 点击列表播放",
            math.floor(pane_left+pane_width/2), math.floor(panel_bottom-opts.footer_height/2), sidebar_font)
    end

    overlay.res_x = width
    overlay.res_y = height
    overlay.z = 80
    -- Each positioned element must be its own ASS event. In one event libass
    -- only honors a single position, which collapses the labels and rows.
    overlay.data = table.concat(parts, "\n")
    overlay:update()
    update_interaction_area(width, panel_bottom, dragging or handle_hovered)
    mp.enable_key_bindings(section, "allow-hide-cursor")
    sidebar_visible = true
end

request_render = function()
    if render_pending then
        return
    end
    render_pending = true
    mp.add_timeout(0.01, render)
end

local function scroll(direction)
    if not sidebar_visible then
        return
    end
    local max_start = math.max(1, #rows)
    local height = 0
    for index = #rows, 1, -1 do
        if height + rows[index].height > content_bottom - opts.header_height then break end
        height = height + rows[index].height
        max_start = index
    end
    visible_start = clamp(visible_start + direction * opts.scroll_rows, 1, max_start)
    refresh_hover = true
    request_render()
end

local function toggle_cover_preview()
    preview_open = not preview_open
    preview_hover_id = nil
    request_render()
end

local function play_clicked_row()
    if not sidebar_visible then
        return
    end
    -- Ignore a click on a stale frame while MPV is replacing/reordering entries.
    if rows_dirty then request_render(); return end
    local mouse_x, mouse_y = get_mouse_position()
    if not mouse_y or not mouse_x or mouse_x < pane_left or mouse_x > window_width then
        return
    end
    if mouse_y < opts.header_height then
        if mouse_x and mouse_x >= window_width - 48 then
            sidebar_collapsed = true
            hide_sidebar()
        elseif enhanced and opts.show_covers and mouse_x >= window_width - 94 then
            toggle_cover_preview()
        end
        return
    end
    if mouse_x >= list_right then return end -- The preview never starts playback.
    for _, hit in ipairs(hit_rows) do
        if mouse_y >= hit.top and mouse_y < hit.bottom then
            local row = hit.row
            preview_hover_id = playlist_snapshot[row.index].id
            if row.kind == "group" then
                if mouse_x >= list_right - 44 then
                    collapsed_groups[row.group.key] = not collapsed_groups[row.group.key]
                    rows_dirty = true
                    request_render()
                    return
                end
                collapsed_groups[row.group.key] = nil
                rows_dirty = true
                request_render()
            end
            local index = row.index
            mp.commandv("playlist-play-index", index - 1)
            return
        end
    end
end

local function show_sidebar()
    sidebar_collapsed = false
    explicitly_opened = true
    mp.commandv("script-message-to", "javboss_highlight_sidebar", "hide")
    request_render()
end

local function hide_sidebar_by_user()
    sidebar_collapsed = true
    hide_sidebar()
end

local function toggle_sidebar()
    if sidebar_visible and not sidebar_collapsed then
        hide_sidebar_by_user()
    else
        show_sidebar()
    end
end

local function begin_click()
    if not sidebar_visible then
        return
    end
    local mouse_x, mouse_y = get_mouse_position()
    if mouse_x and math.abs(mouse_x - pane_left) <= opts.resize_handle_width then
        dragging = true
        handle_hovered = true
        handle_mouse_y = mouse_y
        click_armed = false
        update_interaction_area(window_width, window_height, true)
        request_render()
        return
    end
    if not mouse_x or mouse_x < pane_left then
        click_armed = false
        return
    end
    click_armed = true
end

local function end_click()
    if dragging then
        dragging = false
        click_armed = false
        request_render()
        return
    end
    if click_armed then
        click_armed = false
        play_clicked_row()
    end
end

local function resize_from_mouse()
    if not dragging then
        return
    end
    local mouse_x = select(1, get_mouse_position())
    if not mouse_x or window_width <= 0 then
        return
    end
    local maximum = math.max(1, math.floor(window_width * opts.max_width_ratio))
    local minimum = math.min(opts.min_width, maximum)
    maximum = math.max(1, maximum - preview_width)
    minimum = math.min(minimum, maximum)
    local next_width = clamp(window_width - preview_width - mouse_x, minimum, maximum)
    if math.abs(next_width - current_width) >= 1 then
        current_width = next_width
        request_render()
    end
end

local function cancel_drag()
    local was_active = dragging or handle_hovered or preview_hover_id ~= nil
    preview_hover_id = nil
    handle_hovered = false
    handle_mouse_y = nil
    if dragging then
        dragging = false
        click_armed = false
    end
    update_interaction_area(window_width, window_height, false)
    if was_active then
        request_render()
    end
end

local function handle_mouse_move()
    local mouse_x, mouse_y = get_mouse_position()
    if dragging then
        handle_mouse_y = mouse_y
        resize_from_mouse()
        return
    end
    if not rows_dirty and preview_width > 0 then
        local previous = preview_hover_id
        update_preview_hover()
        if previous ~= preview_hover_id then request_render() end
    end
    local hovered = mouse_x ~= nil and math.abs(mouse_x - pane_left) <= opts.resize_handle_width
    local changed = hovered ~= handle_hovered
    local moved = hovered and mouse_y and (not handle_mouse_y or math.abs(mouse_y - handle_mouse_y) >= 2)
    if changed or moved then
        handle_hovered = hovered
        handle_mouse_y = hovered and mouse_y or nil
        if changed then
            update_interaction_area(window_width, window_height, hovered)
        end
        request_render()
    end
end

-- save-position-on-quit does not persist progress when mpv advances or jumps
-- between playlist entries. Save while the old file is still loaded so the
-- regular resume-playback option can restore it when that entry is opened again.
local function save_position_before_unload()
    if not mp.get_property_native("options/save-position-on-quit", false) then
        return
    end
    if mp.get_property_native("eof-reached", false) then
        return
    end
    if not mp.get_property("path") then
        return
    end
    mp.commandv("write-watch-later-config")
end

mp.set_key_bindings({
    {"mbtn_left", end_click, begin_click},
    {"mouse_move", handle_mouse_move},
    {"mouse_leave", cancel_drag},
    {"wheel_up", function() scroll(-1) end},
    {"wheel_down", function() scroll(1) end},
}, section, "force")
mp.disable_key_bindings(section)

for _, property in ipairs({"playlist-pos", "fullscreen", "window-maximized", "osd-dimensions", "display-hidpi-scale", "display-names", "user-data/javboss/window-dock-layout"}) do
    mp.observe_property(property, "native", request_render)
end
for _, property in ipairs({"playlist", "user-data/javboss/playlist-titles", "user-data/javboss/playlist-highlights", "user-data/javboss/playlist-visuals"}) do
    mp.observe_property(property, "native", function() rows_dirty = true; request_render() end)
end

mp.register_script_message("covers-ready", function(version, data)
    if version ~= generation then return end
    local result = utils.parse_json(data)
    if type(result) ~= "table" then return end
    for key, cover in pairs(result) do
        if cover == false or (type(cover) == "table" and type(cover.path) == "string"
            and type(cover.width) == "number" and type(cover.height) == "number"
            and cover.width >= 1 and cover.width <= 512 and cover.width % 1 == 0
            and cover.height >= 1 and cover.height <= 512 and cover.height % 1 == 0) then images[key] = cover end
    end
    request_render()
end)

mp.register_script_message("thumbfast-info", function(data)
    local info = utils.parse_json(data)
    local id = type(info) == "table" and tonumber(info.overlay_id)
    if id and id >= 0 and id <= 63 and id ~= thumbnail_overlay_id then
        clear_bitmaps()
        thumbnail_overlay_id = id
        request_render()
    end
end)

mp.add_hook("on_unload", 50, save_position_before_unload)
mp.register_script_message("show", show_sidebar)
mp.register_script_message("hide", hide_sidebar_by_user)
mp.register_script_message("toggle", toggle_sidebar)
mp.register_script_message("toggle-cover-preview", toggle_cover_preview)
mp.register_event("file-loaded", request_render)
mp.register_event("shutdown", function()
    hide_sidebar()
    overlay:remove()
end)

request_render()
