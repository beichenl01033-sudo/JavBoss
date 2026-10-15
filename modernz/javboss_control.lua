local mp = require "mp"
local utils = require "mp.utils"
local options = require "mp.options"

local input_ok, input = pcall(require, "mp.input")

local opts = {
    duplicate_window = 2,
}
options.read_options(opts, "javboss_control")

local highlights = {}
local native_chapters = {}
local highlight_path = nil
local clip_a = nil
local clip_b = nil
local clip_request_sequence = 0

local function osd(message, duration)
    mp.osd_message(message, duration or 2.5)
end

local function format_time(seconds)
    seconds = math.max(0, tonumber(seconds) or 0)
    local total = math.floor(seconds + 0.5)
    local hours = math.floor(total / 3600)
    local minutes = math.floor((total % 3600) / 60)
    local secs = total % 60
    if hours > 0 then
        return string.format("%02d:%02d:%02d", hours, minutes, secs)
    end
    return string.format("%02d:%02d", minutes, secs)
end

local function filename_time(seconds)
    seconds = math.max(0, tonumber(seconds) or 0)
    local total = math.floor(seconds + 0.5)
    return string.format(
        "%02dh%02dm%02ds",
        math.floor(total / 3600),
        math.floor((total % 3600) / 60),
        total % 60
    )
end

local function copy_chapter(chapter)
    return { time = tonumber(chapter.time) or 0, title = tostring(chapter.title or "") }
end

local function sort_highlights()
    table.sort(highlights, function(a, b)
        if a.time == b.time then return tostring(a.label or "") < tostring(b.label or "") end
        return a.time < b.time
    end)
end

local function publish_highlights()
    sort_highlights()
    local chapters = {}
    for _, chapter in ipairs(native_chapters) do
        chapters[#chapters + 1] = copy_chapter(chapter)
    end
    for index, item in ipairs(highlights) do
        local label = tostring(item.label or "")
        if label == "" then label = "亮点 " .. index end
        chapters[#chapters + 1] = { time = item.time, title = "★ " .. label }
    end
    table.sort(chapters, function(a, b) return a.time < b.time end)
    mp.set_property_native("chapter-list", chapters)
    mp.set_property_number("user-data/javboss/highlight-count", #highlights)
    mp.set_property_native("user-data/javboss/highlights", highlights)
end

local function save_highlights()
    if not highlight_path then return false end
    local file, err = io.open(highlight_path, "wb")
    if not file then
        mp.msg.error("open highlight file: " .. tostring(err))
        return false
    end
    file:write(utils.format_json({ version = 1, items = highlights }))
    file:write("\n")
    file:close()
    return true
end

local function resolve_highlight_path()
    local screenshot_dir = mp.get_property("screenshot-directory")
    if not screenshot_dir or screenshot_dir == "" then return nil end
    local video_dir = select(1, utils.split_path(screenshot_dir:gsub("[\\/]$", "")))
    if not video_dir or video_dir == "" then return nil end
    return utils.join_path(video_dir, "highlights.json")
end

local function load_highlights()
    highlight_path = resolve_highlight_path()
    highlights = {}
    if highlight_path then
        local file = io.open(highlight_path, "rb")
        if file then
            local parsed = utils.parse_json(file:read("*a") or "")
            file:close()
            local items = type(parsed) == "table" and parsed.items or nil
            if type(items) == "table" then
                for _, item in ipairs(items) do
                    local time = tonumber(item.time)
                    if time and time >= 0 then
                        highlights[#highlights + 1] = {
                            time = time,
                            label = tostring(item.label or ""),
                        }
                    end
                end
            end
        end
    end
    publish_highlights()
end

local function current_time()
    return mp.get_property_number("time-pos")
end

local function nearest_highlight(time)
    local best_index = nil
    local best_distance = math.huge
    for index, item in ipairs(highlights) do
        local distance = math.abs(item.time - time)
        if distance < best_distance then
            best_index = index
            best_distance = distance
        end
    end
    return best_index, best_distance
end

local function add_highlight()
    local time = current_time()
    if not time or not highlight_path then
        osd("当前视频无法保存亮点")
        return
    end
    local _, distance = nearest_highlight(time)
    if distance <= opts.duplicate_window then
        osd("这里已经有亮点")
        return
    end
    highlights[#highlights + 1] = { time = time, label = "亮点 " .. (#highlights + 1) }
    if save_highlights() then
        publish_highlights()
        osd("已添加亮点 " .. format_time(time))
    else
        table.remove(highlights)
        osd("保存亮点失败")
    end
end

local function delete_highlight(index)
    local item = highlights[index]
    if not item then return end
    table.remove(highlights, index)
    if save_highlights() then
        publish_highlights()
        osd("已删除亮点 " .. format_time(item.time))
    end
end

local function rename_highlight(index)
    if not input_ok or not highlights[index] then return end
    local item = highlights[index]
    input.get({
        prompt = "亮点名称:",
        default_text = tostring(item.label or ""),
        submit = function(value)
            value = tostring(value or ""):gsub("^%s+", ""):gsub("%s+$", "")
            if value == "" then return end
            item.label = value
            if save_highlights() then publish_highlights() end
        end,
    })
end

local function choose_highlight(prompt, callback)
    if not input_ok then
        osd("当前 MPV 不支持亮点管理菜单")
        return
    end
    if #highlights == 0 then
        osd("暂无亮点")
        return
    end
    local items = {}
    for index, item in ipairs(highlights) do
        items[index] = format_time(item.time) .. "  " .. tostring(item.label or ("亮点 " .. index))
    end
    input.select({ prompt = prompt, items = items, submit = callback })
end

local function confirm_delete_highlight(index)
    index = tonumber(index)
    local target = index and highlights[index] or nil
    if not target then return end
    if not input_ok then
        delete_highlight(index)
        return
    end
    input.select({
        prompt = "确认删除 " .. format_time(target.time) .. "?",
        items = { "取消", "确认删除" },
        submit = function(confirm_index)
            if confirm_index == 2 then delete_highlight(index) end
        end,
    })
end

local function manage_highlights()
    if not input_ok then
        osd("当前 MPV 不支持亮点管理菜单")
        return
    end
    if #highlights == 0 then
        osd("暂无亮点，左键点击亮点按钮即可添加")
        return
    end
    local items = {}
    for index, item in ipairs(highlights) do
        items[#items + 1] = "跳转  " .. format_time(item.time) .. "  " .. tostring(item.label or ("亮点 " .. index))
    end
    items[#items + 1] = "重命名亮点…"
    items[#items + 1] = "删除亮点…"
    if #highlights > 1 then items[#items + 1] = "清空全部亮点…" end
    input.select({
        prompt = "亮点管理",
        items = items,
        submit = function(index)
            if index <= #highlights then
                mp.commandv("seek", highlights[index].time, "absolute+exact")
                return
            end
            local action = items[index]
            if action == "重命名亮点…" then
                choose_highlight("选择要重命名的亮点", rename_highlight)
            elseif action == "删除亮点…" then
                choose_highlight("选择要删除的亮点", function(delete_index)
                    confirm_delete_highlight(delete_index)
                end)
            elseif action == "清空全部亮点…" then
                input.select({
                    prompt = "确认清空全部亮点?",
                    items = { "取消", "确认清空" },
                    submit = function(confirm_index)
                        if confirm_index == 2 then
                            highlights = {}
                            if save_highlights() then
                                publish_highlights()
                                osd("已清空全部亮点")
                            end
                        end
                    end,
                })
            end
        end,
    })
end

local function clear_clip(show_message)
    clip_a = nil
    clip_b = nil
    mp.set_property("ab-loop-a", "no")
    mp.set_property("ab-loop-b", "no")
    mp.set_property("user-data/javboss/clip-a", "")
    mp.set_property("user-data/javboss/clip-b", "")
    if show_message then osd("已取消剪辑区间") end
end

local function publish_clip()
    mp.set_property("user-data/javboss/clip-a", clip_a and tostring(clip_a) or "")
    mp.set_property("user-data/javboss/clip-b", clip_b and tostring(clip_b) or "")
end

local function clip_supported()
    local path = mp.get_property("path", "")
    if path == "" or path:match("^[%a][%w+.-]*://") then return false end
    return utils.file_info(path) ~= nil
end

local function export_clip()
    if not clip_a or not clip_b or clip_b <= clip_a then
        osd("请先选择有效的剪辑区间")
        return
    end
    if not clip_supported() then
        osd("远程视频暂不支持剪辑")
        return
    end
    local source = mp.get_property("path")
    clip_request_sequence = clip_request_sequence + 1
    local payload = utils.format_json({
        request_id = tostring(os.time()) .. "-" .. tostring(clip_request_sequence),
        source = source,
        start_seconds = clip_a,
        end_seconds = clip_b,
    })
    mp.set_property("user-data/javboss/clip-request", payload)
    clear_clip(false)
    osd("剪辑任务已提交到 JavBoss，可关闭播放器", 5)
end

local function clip_menu()
    if not input_ok or not clip_a or not clip_b then
        osd("请先设置剪辑开始点和结束点")
        return
    end
    input.select({
        prompt = "剪辑区间 " .. format_time(clip_a) .. " - " .. format_time(clip_b),
        items = { "快速无损导出", "循环预览", "重新选择", "取消选择" },
        submit = function(index)
            if index == 1 then
                export_clip()
            elseif index == 2 then
                mp.set_property_number("ab-loop-a", clip_a)
                mp.set_property_number("ab-loop-b", clip_b)
                mp.commandv("seek", clip_a, "absolute+exact")
                mp.set_property_bool("pause", false)
            elseif index == 3 then
                clear_clip(false)
                local time = current_time()
                if time then
                    clip_a = time
                    mp.set_property_number("ab-loop-a", clip_a)
                    publish_clip()
                    osd("剪辑开始点 " .. format_time(clip_a))
                end
            elseif index == 4 then
                clear_clip(true)
            end
        end,
    })
end

local function has_track(track_type)
    for _, track in ipairs(mp.get_property_native("track-list", {}) or {}) do
        if track.type == track_type then return true end
    end
    return false
end

local function speed_menu()
    if not input_ok then return end
    local values = { 0.5, 0.75, 1, 1.25, 1.5, 2 }
    local items = {}
    local current = mp.get_property_number("speed", 1) or 1
    for index, value in ipairs(values) do
        local label = string.format("%gx", value)
        if math.abs(current - value) < 0.001 then label = "✓ " .. label end
        items[index] = label
    end
    input.select({
        prompt = "播放速度",
        items = items,
        submit = function(index)
            local value = values[index]
            if value then mp.set_property_number("speed", value) end
        end,
    })
end

local function more_menu()
    if not input_ok then
        osd("当前 MPV 不支持更多菜单")
        return
    end
    local items = {}
    local actions = {}
    local function add(label, action)
        items[#items + 1] = label
        actions[#actions + 1] = action
    end

    if has_track("audio") then
        add("音轨…", function() mp.commandv("script-binding", "select/select-aid") end)
    end
    if has_track("sub") then
        add("字幕…", function() mp.commandv("script-binding", "select/select-sid") end)
    end
    local speed = mp.get_property_number("speed", 1) or 1
    add(string.format("倍速… (%gx)", speed), speed_menu)
    local ontop = mp.get_property_native("ontop", false)
    add(ontop and "取消窗口置顶" or "窗口置顶", function()
        mp.set_property_bool("ontop", not mp.get_property_native("ontop", false))
    end)
    add("媒体信息", function() mp.commandv("script-binding", "stats/display-page-1-toggle") end)

    input.select({
        prompt = "更多",
        items = items,
        submit = function(index)
            local action = actions[index]
            if action then action() end
        end,
    })
end

local function clip_click()
    if not clip_supported() then
        osd("远程视频暂不支持剪辑")
        return
    end
    local time = current_time()
    if not time then return end
    if not clip_a or clip_b then
        clear_clip(false)
        clip_a = time
        mp.set_property_number("ab-loop-a", clip_a)
        publish_clip()
        osd("剪辑开始点 " .. format_time(clip_a))
        return
    end
    clip_b = time
    if clip_b < clip_a then clip_a, clip_b = clip_b, clip_a end
    mp.set_property_number("ab-loop-a", clip_a)
    mp.set_property_number("ab-loop-b", clip_b)
    publish_clip()
    osd("剪辑结束点 " .. format_time(clip_b))
    clip_menu()
end

mp.register_script_message("add-highlight", add_highlight)
mp.register_script_message("manage-highlights", manage_highlights)
mp.register_script_message("rename-highlight", function(index) rename_highlight(tonumber(index)) end)
mp.register_script_message("confirm-delete-highlight", confirm_delete_highlight)
mp.register_script_message("clear-highlights", function()
    highlights = {}
    if save_highlights() then
        publish_highlights()
        osd("已清空全部亮点")
    end
end)
mp.register_script_message("clip-click", clip_click)
mp.register_script_message("clip-menu", clip_menu)
mp.register_script_message("export-clip", export_clip)
mp.register_script_message("clear-clip", function() clear_clip(true) end)
mp.register_script_message("more-menu", more_menu)

mp.register_event("file-loaded", function()
    local chapters = mp.get_property_native("chapter-list", {}) or {}
    native_chapters = {}
    for _, chapter in ipairs(chapters) do
        if not tostring(chapter.title or ""):match("^★ ") then
            native_chapters[#native_chapters + 1] = copy_chapter(chapter)
        end
    end
    clear_clip(false)
    load_highlights()
end)

mp.register_event("end-file", function()
    highlights = {}
    native_chapters = {}
    highlight_path = nil
    clear_clip(false)
    mp.set_property_number("user-data/javboss/highlight-count", 0)
    mp.set_property_native("user-data/javboss/highlights", {})
end)
