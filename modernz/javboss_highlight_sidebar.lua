local mp = require "mp"
local options = require "mp.options"

local opts = {
    enabled = true,
    width = 320,
    max_width_ratio = 0.58,
    header_height = 58,
    footer_height = 34,
    row_height = 42,
    font_size = 17,
    font = "auto",
    scroll_rows = 3,
    hide_fullscreen = true,
    title = "亮点",
}

options.read_options(opts, "javboss_highlight_sidebar")

local function resolve_font()
    if opts.font and opts.font ~= "" and opts.font ~= "auto" then return opts.font end
    local platform = mp.get_property("platform") or ""
    if platform == "windows" then return "Microsoft YaHei UI" end
    if platform == "darwin" then return "PingFang SC" end
    return "Noto Sans CJK SC"
end

local sidebar_font = resolve_font():gsub("[\\{}]", "")
local section = "javboss-highlight-sidebar"
local width_property = "user-data/javboss/playlist-sidebar-width"
local overlay = mp.create_osd_overlay("ass-events")
local visible_start = 1
local visible_rows = 0
local render_pending = false
local sidebar_visible = false
local sidebar_collapsed = true
local pane_left = 0
local window_width = 0
local window_height = 0
local scale_x = 1
local scale_y = 1
local click_armed = false

local function clamp(value, minimum, maximum)
    return math.max(minimum, math.min(maximum, value))
end

local function get_mouse_position()
    local x, y = mp.get_mouse_pos()
    return x and math.floor(x / scale_x), y and math.floor(y / scale_y)
end

local function ass_escape(value)
    return tostring(value or "")
        :gsub("\\", "\\\\")
        :gsub("{", "\\{")
        :gsub("}", "\\}")
        :gsub("[\r\n]+", " ")
end

local function format_time(seconds)
    seconds = math.max(0, tonumber(seconds) or 0)
    local total = math.floor(seconds + 0.5)
    local hours = math.floor(total / 3600)
    local minutes = math.floor((total % 3600) / 60)
    local secs = total % 60
    if hours > 0 then return string.format("%02d:%02d:%02d", hours, minutes, secs) end
    return string.format("%02d:%02d", minutes, secs)
end

local function highlights()
    local items = mp.get_property_native("user-data/javboss/highlights", {}) or {}
    table.sort(items, function(a, b) return (tonumber(a.time) or 0) < (tonumber(b.time) or 0) end)
    return items
end

local function publish_width(value)
    local current = mp.get_property_number(width_property, 0) or 0
    if math.abs(current - value) > 0.5 then mp.set_property_number(width_property, value) end
end

local function hide_sidebar()
    if sidebar_visible then
        overlay:remove()
        mp.disable_key_bindings(section)
        mp.set_mouse_area(0, 0, 0, 0, section)
        sidebar_visible = false
    end
    publish_width(0)
    mp.set_property_bool("user-data/javboss/highlight-sidebar-open", false)
end

local function append_rect(parts, x1, y1, x2, y2, color, alpha)
    parts[#parts + 1] = string.format(
        "{\\an7\\pos(0,0)\\bord0\\shad0\\1c&H%s&\\1a&H%s&\\p1}m %d %d l %d %d %d %d %d %d{\\p0}",
        color, alpha, x1, y1, x2, y1, x2, y2, x1, y2
    )
end

local function render()
    render_pending = false
    if not opts.enabled or sidebar_collapsed then
        hide_sidebar()
        return
    end
    local fullscreen = mp.get_property_native("fullscreen", false)
    local pixel_width, pixel_height = mp.get_osd_size()
    if not pixel_width or not pixel_height or pixel_width < 1 or pixel_height < 1
        or (opts.hide_fullscreen and fullscreen) then
        hide_sidebar()
        return
    end

    local dpi_scale = mp.get_property_number("display-hidpi-scale", 1)
    if not dpi_scale or dpi_scale ~= dpi_scale or dpi_scale <= 0 or dpi_scale == math.huge then
        dpi_scale = 1
    end
    local width = math.max(1, math.floor(pixel_width / dpi_scale + 0.5))
    local height = math.max(1, math.floor(pixel_height / dpi_scale + 0.5))
    scale_x, scale_y = pixel_width / width, pixel_height / height
    local pane_width = math.max(1, math.min(opts.width, math.floor(width * opts.max_width_ratio)))
    pane_left = width - pane_width
    window_width = width
    window_height = height
    visible_rows = math.max(1, math.floor((height - opts.header_height - opts.footer_height) / opts.row_height))

    local items = highlights()
    visible_start = clamp(visible_start, 1, math.max(1, #items - visible_rows + 1))
    publish_width(pane_width * scale_x)

    local parts = {}
    append_rect(parts, pane_left, 0, width, height, "17191F", "08")
    append_rect(parts, pane_left, opts.header_height - 1, width, opts.header_height, "424650", "00")
    parts[#parts + 1] = string.format(
        "{\\an4\\pos(%d,%d)\\bord0\\shad0\\fs25\\b1\\fn%s\\1c&HFFFFFF&}%s",
        pane_left + 18, math.floor(opts.header_height / 2), sidebar_font, ass_escape(opts.title)
    )
    parts[#parts + 1] = string.format(
        "{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs26\\b0\\fn%s\\1c&HF0A34A&}+",
        width - 68, math.floor(opts.header_height / 2), sidebar_font
    )
    parts[#parts + 1] = string.format(
        "{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs24\\b0\\fn%s\\1c&HCDD0D7&}×",
        width - 22, math.floor(opts.header_height / 2), sidebar_font
    )

    if #items == 0 then
        parts[#parts + 1] = string.format(
            "{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs17\\fn%s\\1c&H9297A2&}暂无亮点，点击右上角 ＋ 添加",
            pane_left + math.floor(pane_width / 2), math.floor(height / 2), sidebar_font
        )
    else
        local last_visible = math.min(#items, visible_start + visible_rows - 1)
        for index = visible_start, last_visible do
            local item = items[index]
            local row = index - visible_start
            local top = opts.header_height + row * opts.row_height
            local text_y = top + math.floor(opts.row_height / 2)
            if row % 2 == 1 then
                append_rect(parts, pane_left + 8, top + 2, width - 8, top + opts.row_height - 2, "20232A", "30")
            end
            parts[#parts + 1] = string.format(
                "{\\an4\\pos(%d,%d)\\bord0\\shad0\\fs15\\b1\\fn%s\\1c&HF0A34A&}%s",
                pane_left + 18, text_y, sidebar_font, format_time(item.time)
            )
            parts[#parts + 1] = string.format(
                "{\\an4\\pos(%d,%d)\\clip(%d,%d,%d,%d)\\bord0\\shad0\\fs%d\\fn%s\\1c&HCDD0D7&}%s",
                pane_left + 86, text_y, pane_left + 82, top, width - 48, top + opts.row_height,
                opts.font_size, sidebar_font, ass_escape(item.label or ("亮点 " .. index))
            )
            parts[#parts + 1] = string.format(
                "{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs20\\fn%s\\1c&H777D88&}×",
                width - 24, text_y, sidebar_font
            )
        end
    end

    append_rect(parts, pane_left, height - opts.footer_height, width, height, "111319", "10")
    parts[#parts + 1] = string.format(
        "{\\an5\\pos(%d,%d)\\bord0\\shad0\\fs13\\fn%s\\1c&H9297A2&}点击跳转 · 右键重命名 · × 删除",
        pane_left + math.floor(pane_width / 2), height - math.floor(opts.footer_height / 2), sidebar_font
    )

    overlay.res_x = width
    overlay.res_y = height
    overlay.z = 81
    overlay.data = table.concat(parts, "\n")
    overlay:update()
    mp.set_mouse_area(math.floor(pane_left * scale_x), 0, pixel_width, pixel_height, section)
    mp.enable_key_bindings(section, "allow-hide-cursor")
    sidebar_visible = true
    mp.set_property_bool("user-data/javboss/highlight-sidebar-open", true)
end

local function request_render()
    if render_pending then return end
    render_pending = true
    mp.add_timeout(0.01, render)
end

local function row_at_mouse()
    if not sidebar_visible then return nil end
    local mouse_x, mouse_y = get_mouse_position()
    if not mouse_x or not mouse_y or mouse_x < pane_left then return nil end
    if mouse_y < opts.header_height or mouse_y >= window_height - opts.footer_height then return nil end
    local row = math.floor((mouse_y - opts.header_height) / opts.row_height)
    local index = visible_start + row
    if index < 1 or index > #highlights() then return nil end
    return index, mouse_x
end

local function begin_click()
    click_armed = sidebar_visible
end

local function end_click()
    if not click_armed then return end
    click_armed = false
    local mouse_x, mouse_y = get_mouse_position()
    if not mouse_x or not mouse_y then return end
    if mouse_y < opts.header_height then
        if mouse_x >= window_width - 46 then
            sidebar_collapsed = true
            hide_sidebar()
        elseif mouse_x >= window_width - 92 then
            mp.commandv("script-message-to", "javboss_control", "add-highlight")
        end
        return
    end
    local index, row_x = row_at_mouse()
    if not index then return end
    if row_x >= window_width - 46 then
        mp.commandv("script-message-to", "javboss_control", "confirm-delete-highlight", tostring(index))
        return
    end
    local item = highlights()[index]
    if item then mp.commandv("seek", tostring(item.time), "absolute+exact") end
end

local function rename_clicked_row()
    local index = row_at_mouse()
    if index then
        mp.commandv("script-message-to", "javboss_control", "rename-highlight", tostring(index))
    end
end

local function scroll(direction)
    if not sidebar_visible then return end
    local items = highlights()
    visible_start = clamp(visible_start + direction * opts.scroll_rows, 1, math.max(1, #items - visible_rows + 1))
    request_render()
end

local function show_sidebar()
    sidebar_collapsed = false
    mp.commandv("script-message-to", "playlist_sidebar", "hide")
    request_render()
end

local function hide_sidebar_by_user()
    sidebar_collapsed = true
    hide_sidebar()
end

local function toggle_sidebar()
    if sidebar_visible and not sidebar_collapsed then hide_sidebar_by_user() else show_sidebar() end
end

mp.set_key_bindings({
    { "mbtn_left", end_click, begin_click },
    { "mbtn_right", rename_clicked_row },
    { "wheel_up", function() scroll(-1) end },
    { "wheel_down", function() scroll(1) end },
}, section, "force")
mp.disable_key_bindings(section)

for _, property in ipairs({
    "fullscreen",
    "osd-dimensions",
    "display-hidpi-scale",
    "user-data/javboss/highlights",
}) do
    mp.observe_property(property, "native", request_render)
end

mp.register_script_message("show", show_sidebar)
mp.register_script_message("hide", hide_sidebar_by_user)
mp.register_script_message("toggle", toggle_sidebar)
mp.register_event("shutdown", function()
    hide_sidebar()
    overlay:remove()
end)
