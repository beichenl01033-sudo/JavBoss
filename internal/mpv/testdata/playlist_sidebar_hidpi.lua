local real_mp = require "mp"
local real_options = require "mp.options"
local real_utils = require "mp.utils"
local sidebar_path = real_mp.get_opt("sidebar-script")

local function near(actual, expected)
    assert(math.abs(actual - expected) < 0.001, string.format("got %s, expected %s", actual, expected))
end

local function harness(scale, width, height, position, overrides)
    local state = {
        width = width or 1000 * scale,
        height = height or 600 * scale,
        properties = {
            ["display-hidpi-scale"] = scale,
            ["playlist-pos"] = position or 0,
            ["video-margin-ratio-right"] = 0,
            playlist = {},
        },
        observers = {},
        bindings = {},
        timers = {},
        commands = {},
        messages = {},
        bitmaps = {},
        active_bitmaps = {},
    }
    for i = 1, 40 do
        state.properties.playlist[i] = {id = i, title = "entry-" .. i}
    end
    local overlay = {}
    function overlay:update()
        self.visible = true
    end
    function overlay:remove()
        self.visible = false
    end
    state.overlay = overlay
    local function property(name, fallback)
        if state.properties[name] == nil then return fallback end
        return state.properties[name]
    end
    package.loaded["mp"] = {
        get_property = property,
        get_property_number = property,
        get_property_native = property,
        set_property_number = function(name, value) state.properties[name] = value end,
        set_property_native = function(name, value) state.properties[name] = value end,
        create_osd_overlay = function() return overlay end,
        get_osd_size = function() return state.width, state.height end,
        get_mouse_pos = function() return state.mouse_x, state.mouse_y end,
        set_mouse_area = function(x1, y1, x2, y2) state.area = {x1, y1, x2, y2} end,
        enable_key_bindings = function() state.enabled = true end,
        disable_key_bindings = function() state.enabled = false end,
        set_key_bindings = function(bindings)
            for _, binding in ipairs(bindings) do state.bindings[binding[1]] = binding end
        end,
        observe_property = function(name, _, callback) state.observers[name] = callback end,
        add_timeout = function(_, callback) state.timers[#state.timers + 1] = callback end,
        commandv = function(...)
            local command = {...}
            state.commands[#state.commands + 1] = command
            if command[1] == "overlay-remove" then state.active_bitmaps[command[2]] = nil end
        end,
        command_native = function(command)
            state.bitmaps[#state.bitmaps+1] = command
            state.active_bitmaps[command.id] = command
            return true
        end,
        add_hook = function() end,
        register_script_message = function(name, callback) state.messages[name] = callback end,
        register_event = function() end,
    }
    package.loaded["mp.options"] = {read_options = function(options)
        options.dock_sidebar = false -- Legacy overlay behavior is tested separately.
        for key, value in pairs(overrides or {}) do options[key] = value end
    end}
    package.loaded["mp.utils"] = real_utils
    dofile(sidebar_path)
    package.loaded["mp"] = real_mp
    package.loaded["mp.options"] = real_options

    function state:flush()
        while #self.timers > 0 do
            local timers = self.timers
            self.timers = {}
            for _, callback in ipairs(timers) do callback() end
        end
    end
    function state:mouse(x, y)
        self.mouse_x = (x + 0.25) * self.width / self.overlay.res_x
        self.mouse_y = (y + 0.25) * self.height / self.overlay.res_y
    end
    function state:click(x, y)
        self:mouse(x, y)
        self.bindings.mbtn_left[3]()
        self.bindings.mbtn_left[2]()
        self:flush()
    end
    function state:change(name, value)
        self.properties[name] = value
        assert(self.observers[name], "missing observer: " .. name)()
        self:flush()
    end
    state:flush()
    assert(state.commands[1][1] == "script-message-to")
    assert(state.commands[1][2] == "modernz")
    assert(state.commands[1][3] == "osc-visibility")
    assert(state.commands[1][4] == "always")
    state.commands = {}
    return state
end

local function check_layout(state, logical_width, logical_height, pane_width)
    assert(state.overlay.visible and state.enabled)
    near(state.overlay.res_x, logical_width)
    near(state.overlay.res_y, logical_height)
    local pixel_pane_width = pane_width * state.width / logical_width
    near(state.properties["user-data/javboss/playlist-sidebar-width"], pixel_pane_width)
    near(state.properties["video-margin-ratio-right"], 0)
    near(state.area[3], state.width)
    local panel_bottom = math.max(58 + 34 + 35, logical_height - 96)
    near(state.area[4], math.floor(panel_bottom * state.height / logical_height + 0.5))
    assert(state.overlay.data:find("\\fs18", 1, true), "logical font size changed")
    assert(state.overlay.data:find("\\pos(" .. (logical_width - pane_width + 52) .. ",75)", 1, true))
end

local function run_tests()
    for _, scale in ipairs({1, 1.25, 1.5, 2}) do
        local state = harness(scale)
        check_layout(state, 1000, 600, 320)
        near(state.area[1], math.floor(675 * scale))
        state:click(900, 58 + 35 + 17)
        assert(state.commands[1][1] == "playlist-play-index")
        assert(state.commands[1][2] == 1, "click selected the wrong row")
        state:click(100, 110)
        assert(#state.commands == 1, "video click selected a playlist row")

        state:mouse(680, 200)
        state.bindings.mouse_move[2]()
        state:flush()
        near(state.area[1], 0)
        state.bindings.mbtn_left[3]()
        state:mouse(600, 200)
        state.bindings.mouse_move[2]()
        state.bindings.mbtn_left[2]()
        state:flush()
        near(state.properties["user-data/javboss/playlist-sidebar-width"], 400 * scale)
        near(state.properties["video-margin-ratio-right"], 0)
        assert(#state.commands == 1, "resize played an entry")
        state.bindings.mouse_leave[2]()
        state:flush()
        near(state.area[1], math.floor(595 * scale))
        state.bindings.wheel_down[2]()
        state:flush()
        state:click(900, 75)
        assert(state.commands[2][2] == 3, "scrolled row click is misaligned")

        state:change("fullscreen", true)
        assert(not state.overlay.visible and not state.enabled)
        near(state.properties["user-data/javboss/playlist-sidebar-width"], 0)
        near(state.properties["video-margin-ratio-right"], 0)
    end

    local fractional = harness(1.5, 1501, 901)
    check_layout(fractional, 1001, 601, 320)
    fractional:click(900, 110)
    assert(fractional.commands[1][2] == 1)
    near(fractional.area[1], math.floor(676 * 1501 / 1001))

    check_layout(harness(nil, 1000, 600), 1000, 600, 320)
    for _, scale in ipairs({0, -1, math.huge, 0 / 0}) do
        local state = harness(scale, 1000, 600)
        check_layout(state, 1000, 600, 320)
    end

    local state = harness(1)
    state:mouse(680, 200)
    state.bindings.mbtn_left[3]()
    state:mouse(600, 200)
    state.bindings.mouse_move[2]()
    state:flush()
    state.width, state.height = 2000, 1200
    state:change("display-hidpi-scale", 2)
    check_layout(state, 1000, 600, 400)
    state.bindings.mbtn_left[2]()
    assert(#state.commands == 0, "DPI change retained a stale click")
    state:click(900, 110)
    assert(state.commands[1][2] == 1)
    state.width, state.height = 1000, 600
    state:change("display-hidpi-scale", 1)
    check_layout(state, 1000, 600, 400)

    local current = harness(1, 1000, 1200, 24)
    current:change("display-hidpi-scale", 2)
    assert(current.overlay.data:find("entry-25", 1, true), "DPI change scrolled the current entry out of view")

    local closable = harness(1)
    closable:click(980, 29)
    assert(not closable.overlay.visible and not closable.enabled, "header close button did not collapse the sidebar")
    near(closable.properties["user-data/javboss/playlist-sidebar-width"], 0)

    for _, scale in ipairs({1, 1.25, 2}) do
        local grouped = harness(scale, nil, nil, nil, {cover_preview=false})
        grouped.properties.playlist = {{id=101, filename="a"}, {id=102, filename="a"}, {id=103, filename="b"}}
        grouped.properties["user-data/javboss/playlist-highlights"] = {
            ["101"]={video_id=1, video_title="Video A", time=0, label="First"},
            ["102"]={video_id=1, video_title="Video A", time=12, label="Second"},
            ["103"]={video_id=2, video_title="Video B", time=30, label="Third"},
        }
        grouped:change("user-data/javboss/playlist-visuals", {generation="test", entries={
            ["101"]={key="a", title="Video A"}, ["102"]={key="a", title="Video A"}, ["103"]={key="b", title="Video B"},
        }})
        local _, title_count = grouped.overlay.data:gsub("Video A", "")
        assert(title_count == 1, "repeated the video cover/title for every highlight")
        local request = grouped.properties["user-data/javboss/playlist-cover-request"]
        assert(#request.keys == 2 and request.keys[1] == "a" and request.keys[2] == "b", "did not share artwork between highlights")
        grouped:click(900, 170)
        assert(grouped.commands[#grouped.commands][1] == "playlist-play-index" and grouped.commands[#grouped.commands][2] == 1, "second highlight played the first one")
        local command_count = #grouped.commands
        grouped:click(975, 90)
        assert(#grouped.commands == command_count, "folding a group started playback")
        assert(not grouped.overlay.data:find("Second", 1, true), "group did not fold")
        grouped:change("playlist-pos", 1)
        assert(not grouped.overlay.data:find("Second", 1, true), "advancing a point reopened a manually folded video")
        grouped:click(900, 202)
        assert(grouped.commands[#grouped.commands][2] == 2, "folded groups broke native indices")
        grouped:click(730, 90)
        assert(grouped.commands[#grouped.commands][2] == 0 and grouped.overlay.data:find("Second", 1, true), "cover click did not open/play the first highlight")
        grouped.messages["covers-ready"]("stale", real_utils.format_json({a={path="cover.bgra",width=128,height=128}}))
        grouped:flush()
        assert(#grouped.bitmaps == 0, "accepted artwork from an old playlist")
        grouped.messages["covers-ready"]("test", real_utils.format_json({a={path="cover.bgra",width=128,height=128}, b=false}))
        grouped:flush()
        assert(#grouped.bitmaps == 1, "cover repeated for each highlight")
        local bitmap = grouped.bitmaps[1]
        near(bitmap.x, math.floor(698*scale+0.5))
        near(bitmap.y, math.floor(66*scale+0.5))
        near(bitmap.dw, math.floor(48*scale+0.5))
        assert(bitmap.id ~= 42, "reused timeline thumbnail bitmap ID")
        grouped.messages["thumbfast-info"](real_utils.format_json({overlay_id=0}))
        grouped:flush()
        assert(grouped.bitmaps[#grouped.bitmaps].id ~= 0, "ignored thumbfast's custom bitmap ID")
        grouped:click(980, 29)
        assert(not grouped.overlay.visible, "closing left artwork visible")
        assert(grouped.commands[#grouped.commands][4] == "auto", "closing hid bottom controls")
    end

    local normal = harness(1, nil, nil, nil, {cover_preview=false})
    local entries = {}
    for i=1,40 do entries[tostring(i)]={key="video-"..i, title="Movie "..i} end
    normal:change("user-data/javboss/playlist-visuals", {generation="normal",entries=entries})
    assert(#normal.properties["user-data/javboss/playlist-cover-request"].keys == 6, "requested artwork outside the viewport")
    normal.bindings.wheel_down[2]()
    normal:flush()
    normal:click(900, 90)
    assert(normal.commands[#normal.commands][2] == 3, "normal cover rows lost scroll position")

    for _, scale in ipairs({1, 1.25, 2}) do
        local preview = harness(scale)
        preview:change("user-data/javboss/playlist-visuals", {generation="preview", entries=entries})
        assert(preview.properties["user-data/javboss/playlist-preview-visible"] == true)
        near(preview.properties["user-data/javboss/playlist-sidebar-width"], 560*scale)
        near(preview.properties["video-margin-ratio-right"], 0)
        assert(preview.overlay.data:find("Movie 6", 1, true) and not preview.overlay.data:find("Movie 7", 1, true), "large preview consumed list rows")
        preview:mouse(600, 150)
        preview.bindings.mouse_move[2]()
        preview:flush()
        near(preview.properties["user-data/javboss/playlist-preview-index"], 1)
        assert(#preview.commands == 0, "hover started playback")
        local request = preview.properties["user-data/javboss/playlist-cover-request"]
        assert(request.keys[1] == "video-2", "preview was not prioritized")
        preview:mouse(900, 180)
        preview.bindings.mouse_move[2]()
        preview:flush()
        near(preview.properties["user-data/javboss/playlist-preview-index"], 1)
        preview:click(900, 180)
        assert(#preview.commands == 0, "clicking the preview started playback")
        preview.messages["covers-ready"]("preview", real_utils.format_json({["video-2"]={path="poster.bgra",width=256,height=512}}))
        preview:flush()
        local large, mini
        for _, bitmap in pairs(preview.active_bitmaps) do
            if bitmap.dw > 48*scale then large = bitmap else mini = bitmap end
        end
        assert(large and mini, "large preview and thumbnail did not share artwork")
        near(large.dw, math.floor(144*scale+0.5))
        near(large.dh, math.floor(288*scale+0.5))
        near(mini.dw, math.floor(24*scale+0.5))
        near(mini.dh, math.floor(48*scale+0.5))
        assert(large.id ~= mini.id and large.id ~= 42 and mini.id ~= 42)
        preview:change("playlist-pos", 2)
        near(preview.properties["user-data/javboss/playlist-preview-index"], 2)
        preview:mouse(600, 150)
        preview.bindings.mouse_move[2]()
        preview:flush()
        preview.bindings.mouse_leave[2]()
        preview:flush()
        near(preview.properties["user-data/javboss/playlist-preview-index"], 2)
        preview:click(933, 29)
        assert(preview.properties["user-data/javboss/playlist-preview-visible"] == false)
        near(preview.properties["user-data/javboss/playlist-sidebar-width"], 320*scale)
        assert(preview.overlay.data:find("Movie 6", 1, true), "closing preview changed list density")
        preview:click(900, 150)
        assert(preview.commands[#preview.commands][1] == "playlist-play-index" and preview.commands[#preview.commands][2] == 1, "closing preview broke row hit testing")
        preview:click(933, 29)
        assert(preview.properties["user-data/javboss/playlist-preview-visible"] == true)
        near(preview.properties["video-margin-ratio-right"], 0)
        preview:change("fullscreen", true)
        assert(not preview.overlay.visible and next(preview.active_bitmaps) == nil, "fullscreen retained a preview bitmap")
    end

    local narrow = harness(1, 640, 600)
    narrow:change("user-data/javboss/playlist-visuals", {generation="narrow", entries=entries})
    assert(not narrow.properties["user-data/javboss/playlist-preview-visible"], "preview squeezed a narrow list")
    near(narrow.properties["user-data/javboss/playlist-sidebar-width"], 320)
    narrow.width = 1000
    narrow:change("osd-dimensions", {})
    assert(narrow.properties["user-data/javboss/playlist-preview-visible"], "wider window did not restore preview")
    narrow.height = 300
    narrow:change("osd-dimensions", {})
    assert(not narrow.properties["user-data/javboss/playlist-preview-visible"], "preview overflowed a short window")

    local points = harness(1)
    points.properties.playlist = {{id=101,filename="a"},{id=102,filename="a"},{id=103,filename="b"}}
    points.properties["user-data/javboss/playlist-highlights"] = {
        ["101"]={video_id=1,video_title="Video A",time=0,label="First"},
        ["102"]={video_id=1,video_title="Video A",time=12,label="Second"},
        ["103"]={video_id=2,video_title="Video B",time=30,label="Third"},
    }
    points:change("user-data/javboss/playlist-visuals", {generation="points",entries={
        ["101"]={key="a",title="Video A"},["102"]={key="a",title="Video A"},["103"]={key="b",title="Video B"},
    }})
    points:mouse(600, 170)
    points.bindings.mouse_move[2]()
    points:flush()
    near(points.properties["user-data/javboss/playlist-preview-index"], 1)
    assert(points.properties["user-data/javboss/playlist-cover-request"].keys[1] == "a", "highlight did not preview its own video")
    points:click(600, 170)
    assert(points.commands[#points.commands][2] == 1, "preview broke highlight selection")
    local count = #points.commands
    points:click(735, 90)
    assert(#points.commands == count and not points.overlay.data:find("Second", 1, true), "preview broke group folding")
    points:click(933, 29)
    points:click(900, 170)
    assert(points.commands[#points.commands][2] == 2, "folded preview groups lost native indices")

    local docked = harness(1, 960, 640, 0, {dock_sidebar=true})
    docked:change("user-data/javboss/playlist-visuals", {generation="dock",entries=entries})
    assert(docked.properties["user-data/javboss/window-dock-request"].preferred == 560, "preferred dock width depended on cramped render width")
    assert(docked.properties["video-margin-ratio-right"] > 0, "fallback sidebar covered the video")
    docked.width = 1520
    docked:change("user-data/javboss/window-dock-layout", {expanded=true,controls_width=960,window_width=1520})
    near(docked.properties["user-data/javboss/playlist-sidebar-width"], 560)
    near(docked.properties["video-margin-ratio-right"], 560/1520)
    near(docked.properties["user-data/javboss/window-dock-request"].preferred, 560)
    docked.messages.hide()
    docked:flush()
    near(docked.properties["user-data/javboss/window-dock-request"].preferred, 0)
    near(docked.properties["video-margin-ratio-right"], 0)
end

local ok, err = pcall(run_tests)
if ok then
    real_mp.msg.info("SIDEBAR_HIDPI_TESTS_PASSED")
else
    real_mp.msg.error(tostring(err))
end
real_mp.commandv("quit", ok and 0 or 1)
