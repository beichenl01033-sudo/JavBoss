local real_mp = require "mp"
local file = assert(io.open(real_mp.get_opt("test-script"), "rb"))
local source = file:read("*a")
file:close()
local start = assert(source:find("local function visibility_mode(mode, no_osd)",1,true))
local finish = assert(source:find("local fullscreen_restore_visibility",start,true))
local block = source:sub(start,finish-1) .. "\nreturn visibility_mode"

local function harness(visible, mode)
    local state = { enabled=true, osc_visible=visible, wc_visible=true }
    local user_opts = { visibility=mode }
    local published = {}
    local env = {
        state=state, user_opts=user_opts,
        always_on=function(value) state.osc_visible=value; state.wc_visible=value end,
        hide_wc=function() state.wc_visible=false end,
        enable_osc=function(value) state.enabled=value; if not value then state.osc_visible=false end end,
        reset_timeout=function() state.timeout_reset=true end,
        update_margins=function() end, request_tick=function() end,
        msg={ warn=function(value) error(value) end },
        mp={
            set_property_native=function(name,value) published[name]=value end,
            disable_key_bindings=function() end, get_property=function() return "0" end,
        },
    }
    setmetatable(env,{__index=_G})
    local fn = assert(load(block,"visibility_mode regression","t",env))()
    return fn,state,user_opts,published
end

local function run()
    local fn,state,opts,published=harness(true,"always")
    fn("auto",true)
    assert(state.osc_visible,"closing the sidebar immediately hid the bottom controls")
    assert(state.timeout_reset,"auto-hide kept an expired timeout after closing the sidebar")
    assert(opts.visibility=="auto" and published["user-data/osc/visibility"]=="auto","closing left controls permanently pinned")
    assert(not state.wc_visible,"bottom bar release retained pinned window controls")
    fn("never",true)
    assert(not state.enabled and not state.osc_visible,"explicit never mode stopped working")
    fn,state=harness(false,"always")
    fn("auto",true)
    assert(not state.osc_visible,"release unexpectedly showed an already-hidden control bar")

    local first = assert(source:find("local function control_canvas_width(display_width)",1,true))
    local last = assert(source:find("local function osc_init()",first,true))
    local width, layout = 960, {}
    local environment = {mp={
        get_osd_size=function() return width,640 end,
        get_property_native=function() return layout end,
    }}
    setmetatable(environment,{__index=_G})
    local canvas = assert(load(source:sub(first,last-1).."\nreturn control_canvas_width", "control width", "t", environment))()
    assert(canvas(960)==960,"default control layout changed")
    width,layout=1520,{controls_width=960}
    assert(canvas(1520)==960,"expanding a sidebar recentered the buttons")
    width,layout=960,{controls_width=960}
    assert(canvas(960)==960,"fallback shrink recentered the buttons")
    width,layout=3040,{controls_width=1920}
    assert(canvas(1520)==960,"HiDPI changed the original button coordinates")
    layout={controls_width=-1}
    assert(canvas(1520)==1520,"invalid control width was accepted")
end
local ok,err=pcall(run)
if ok then real_mp.msg.info("CONTROL_VISIBILITY_TESTS_PASSED") else real_mp.msg.error(tostring(err)) end
real_mp.commandv("quit",ok and 0 or 1)
