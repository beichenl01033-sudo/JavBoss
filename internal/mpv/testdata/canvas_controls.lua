local real_mp = require "mp"
local real_options = require "mp.options"
local source = real_mp.get_opt("test-script")
local now, x, y = 0, 300, 200
local properties = { path = "video.mp4", ["input-doubleclick-time"] = 300, ["display-hidpi-scale"] = 1, ["user-data/javboss/playlist-sidebar-width"] = 320 }
local timers, bindings, events, commands = {}, {}, {}, {}
local function property(name, fallback) if properties[name] == nil then return fallback end return properties[name] end
local fake_mp = {
    get_property = property, get_property_number = property, get_property_native = property,
    get_time = function() return now end,
    get_mouse_pos = function() return x, y end,
    get_osd_size = function() return 1000, 700 end,
    add_key_binding = function(_, name, fn) bindings[name] = fn end,
    register_event = function(name, fn) events[name] = events[name] or {}; table.insert(events[name], fn) end,
    register_script_message = function() end, observe_property = function() end,
    add_periodic_timer = function() end,
    add_timeout = function(delay, fn)
        local timer = { due = now + delay, fn = fn }
        function timer:kill() self.killed = true end
        table.insert(timers, timer)
        return timer
    end,
    commandv = function(...) table.insert(commands, {...}) end,
}
package.loaded["mp"] = fake_mp
package.loaded["mp.options"] = { read_options = function() end }
dofile(source)
package.loaded["mp"] = real_mp
package.loaded["mp.options"] = real_options
local function advance(seconds)
    now = now + seconds
    local pending = timers
    timers = {}
    for _, timer in ipairs(pending) do
        if not timer.killed then
            if timer.due <= now then timer.fn() else table.insert(timers, timer) end
        end
    end
end
local function click() bindings["canvas-click"]({event="down"}); bindings["canvas-click"]({event="up"}) end
local function run()
    click()
    assert(#commands == 0, "single click did not wait for double-click recognition")
    advance(0.4)
    assert(#commands == 1 and commands[1][2] == "pause", "canvas single click failed")
    commands = {}
    click(); advance(0.1); bindings["canvas-double-click"](); click(); advance(0.5)
    assert(#commands == 1 and commands[1][2] == "fullscreen", "double-click also toggled pause")
    commands = {}; advance(0.5)
    bindings["canvas-click"]({event="down"}); x = 400; bindings["canvas-click"]({event="up"}); advance(0.4)
    assert(#commands == 0, "dragging toggled pause")
    x = 900; click(); advance(0.4)
    assert(#commands == 0, "sidebar clicks toggled pause")
    bindings["canvas-double-click"]()
    assert(#commands == 0, "sidebar double-click toggled fullscreen")
    x, y = 300, 670; click(); advance(0.4)
    assert(#commands == 0, "control bar clicks toggled pause")
    y = 200; click()
    for _, fn in ipairs(events["start-file"]) do fn() end
    advance(0.4)
    assert(#commands == 0, "a pending click paused a different video")
    bindings["canvas-click"]({event="down"})
    for _, fn in ipairs(events["start-file"]) do fn() end
    bindings["canvas-click"]({event="up"}); advance(0.4)
    assert(#commands == 0, "a held click paused a different video")
    bindings["canvas-click"]({event="down"}); bindings["canvas-click"]({event="up", canceled=true}); advance(0.4)
    assert(#commands == 0, "canceled click toggled pause")
end
local ok, err = pcall(run)
if ok then real_mp.msg.info("CANVAS_CLICK_TESTS_PASSED") else real_mp.msg.error(tostring(err)) end
real_mp.commandv("quit", ok and 0 or 1)
