local mp = require "mp"
local utils = require "mp.utils"
local options = require "mp.options"

local input_ok, input = pcall(require, "mp.input")

local opts = {
    ffmpeg_path = "",
    duplicate_window = 2,
}
options.read_options(opts, "javboss_control")

local highlights = {}
local native_chapters = {}
local highlight_path = nil
local clip_a = nil
local clip_b = nil
local export_running = false

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
                    local target = highlights[delete_index]
                    if not target then return end
                    input.select({
                        prompt = "确认删除 " .. format_time(target.time) .. "?",
                        items = { "取消", "确认删除" },
                        submit = function(confirm_index)
                            if confirm_index == 2 then delete_highlight(delete_index) end
                        end,
                    })
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

local function next_output_path(path, start_time, end_time)
    local dir, filename = utils.split_path(path)
    local extension = filename:match("(%.[^%.]+)$") or ".mkv"
    local stem = filename:sub(1, #filename - #extension)
    local suffix = ".javboss-clip-" .. filename_time(start_time) .. "-" .. filename_time(end_time)
    local candidate = utils.join_path(dir, stem .. suffix .. extension)
    local sequence = 2
    while utils.file_info(candidate) do
        candidate = utils.join_path(dir, stem .. suffix .. string.format("-%02d", sequence) .. extension)
        sequence = sequence + 1
    end
    return candidate, extension
end

local function export_clip()
    if export_running then
        osd("剪辑正在导出")
        return
    end
    if not clip_a or not clip_b or clip_b <= clip_a then
        osd("请先选择有效的剪辑区间")
        return
    end
    if not clip_supported() then
        osd("远程视频暂不支持剪辑")
        return
    end
    if opts.ffmpeg_path == "" or not utils.file_info(opts.ffmpeg_path) then
        osd("未找到 FFmpeg，请先在 JavBoss 工具页面下载")
        return
    end

    local source = mp.get_property("path")
    local output, extension = next_output_path(source, clip_a, clip_b)
    local temp = output .. ".part" .. extension
    os.remove(temp)
    export_running = true
    osd("正在无损导出剪辑…", 4)
    mp.command_native_async({
        name = "subprocess",
        playback_only = false,
        capture_stdout = true,
        capture_stderr = true,
        args = {
            opts.ffmpeg_path,
            "-hide_banner", "-loglevel", "error",
            "-ss", tostring(clip_a),
            "-i", source,
            "-t", tostring(clip_b - clip_a),
            "-map", "0",
            "-c", "copy",
            "-avoid_negative_ts", "make_zero",
            "-y", temp,
        },
    }, function(success, result, error_message)
        export_running = false
        if not success or not result or result.status ~= 0 then
            os.remove(temp)
            mp.msg.error("clip export failed: " .. tostring(error_message or (result and result.stderr)))
            osd("剪辑导出失败")
            return
        end
        local renamed, rename_error = os.rename(temp, output)
        if not renamed then
            os.remove(temp)
            mp.msg.error("rename clip output failed: " .. tostring(rename_error))
            osd("剪辑文件保存失败")
            return
        end
        clear_clip(false)
        osd("剪辑已保存：" .. select(2, utils.split_path(output)), 5)
    end)
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
end)
