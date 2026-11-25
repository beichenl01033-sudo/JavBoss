local real_mp = require "mp"
local source = real_mp.get_opt("test-script")
local props = {path="video.mp4", seekable=true, ["time-pos"]=100, duration=3600, pause=true, hwdec="no", ["hwdec-current"]="no", ["video-format"]="hevc"}
local bindings, events, observers, messages, timers, commands = {}, {}, {}, {}, {}, {}
local selection, notice
local function get(name, fallback) if props[name] == nil then return fallback end return props[name] end
local function set(name, value) props[name] = value end
local fake_mp = {
    get_property=get, get_property_number=get, get_property_native=get, get_property_bool=get,
    set_property=set, set_property_native=set, set_property_number=set, set_property_bool=set,
    get_time=function() return 0 end, get_mouse_pos=function() return 0,0 end, get_osd_size=function() return 1000,700 end,
    commandv=function(...) commands[#commands+1]={...} end,
    add_key_binding=function(_, name, fn, flags) bindings[name]=fn; if name=="adaptive-seek" then assert(flags.complex) end end,
    register_script_message=function(name, fn) messages[name]=fn end,
    register_event=function(name, fn) events[name]=events[name] or {}; table.insert(events[name],fn) end,
    observe_property=function(name, _, fn) observers[name]=observers[name] or {}; table.insert(observers[name],fn) end,
    add_periodic_timer=function(interval, fn)
        local timer={interval=interval, fn=fn}; function timer:kill() self.killed=true end
        timers[#timers+1]=timer; return timer
    end,
    add_timeout=function(_, fn) fn(); return {kill=function() end} end,
    osd_message=function(text) notice=text end,
}
package.loaded["mp"]=fake_mp
package.loaded["mp.options"]={read_options=function() end}
package.loaded["mp.input"]={select=function(value) selection=value end, terminate=function() local current=selection; selection=nil; if current.closed then current.closed() end end}
dofile(source)
local function emit(event, amount, key, canceled)
    bindings["adaptive-seek"]({event=event,arg=amount and tostring(amount),key_name=key or "RIGHT",canceled=canceled})
end
local function tick()
    for _, timer in ipairs(timers) do if not timer.killed and timer.interval==0.125 then timer.fn() end end
end
local function event(name)
    for _, fn in ipairs(events[name] or {}) do fn() end
end
local function observed(name)
    for _, fn in ipairs(observers[name] or {}) do fn(name,props[name]) end
end
local function last(target, mode)
    local cmd=commands[#commands]
    assert(cmd and cmd[1]=="no-osd" and cmd[2]=="seek" and math.abs(cmd[3]-target)<0.001 and cmd[4]==mode, "wrong seek")
end
local function run()
    for _, path in ipairs({"/ssd/video.mp4", "Z:\\media\\video.mp4", "\\\\nas\\media\\video.mp4", "https://server/video"}) do
        props.path=path; props["time-pos"]=100; commands={}
        emit("down",5); assert(#commands==0,"a tap decoded before release")
        emit("up"); assert(#commands==1); last(105,"absolute+exact")
        commands={}; emit("down",1); emit("repeat",1); last(102,"absolute+keyframes")
        props.seeking=true
        for i=1,40 do emit("repeat",1); tick() end
        assert(#commands==1,"queued seeks while decoder/network was busy")
        props.seeking=false; tick(); assert(#commands==2); last(142,"absolute+keyframes")
        tick(); assert(#commands==2,"reissued unchanged target")
        emit("up"); assert(#commands==3); last(142,"absolute+exact")
        tick(); assert(#commands==3 and props.pause,"release changed pause or left timer running")
        assert(props["user-data/javboss/seek-preview"]==nil)
    end
    commands={}; emit("down",5); emit("repeat",5); commands={}
    emit("up",nil,"RIGHT",true); tick(); assert(#commands==0,"cancel committed a stale seek")
    for _, name in ipairs({"start-file","end-file"}) do
        emit("down",5); emit("repeat",5); commands={}; event(name); emit("up"); tick()
        assert(#commands==0,"held key moved a different playlist entry")
    end
    emit("down",5); emit("repeat",5); commands={}; props.focused=false; observed("focused"); emit("up"); tick()
    assert(#commands==0,"lost focus committed seek")
    props.focused=true; props["playlist-pos"]=0; emit("down",5); emit("repeat",5); commands={}
    props["playlist-pos"]=1; emit("up"); tick(); assert(#commands==0,"same-file playlist transition received a stale correction")
    commands={}; emit("down",5); emit("down",-5,"LEFT"); emit("up",nil,"RIGHT",true); emit("up",nil,"LEFT")
    assert(#commands==1); last(95,"absolute+exact")
    commands={}; props["time-pos"]=3599; emit("press",5); last(3599.95,"absolute+exact")
    props["time-pos"]=1; emit("press",-5); last(0,"absolute+exact")
    props.seekable=false; commands={}; emit("down",5); emit("repeat",5); emit("up"); assert(#commands==0)
    props.seekable=true; props["time-pos"]=100; emit("down",5); emit("repeat",5); commands={}
    messages["seek-highlight"]("1"); emit("up"); tick(); assert(#commands==0,"highlight navigation left a pending seek")

    observed("hwdec-current"); assert(props["user-data/javboss/decoder-status"].label=="软件解码")
    props.hwdec="auto"; observed("hwdec"); assert(props["user-data/javboss/decoder-status"].label:find("未使用",1,true))
    props["hwdec-current"]="d3d11va"; observed("hwdec-current")
    assert(props["user-data/javboss/decoder-status"].active=="d3d11va")
    messages["more-menu"](); local found=false
    for i,label in ipairs(selection.items) do if label:find("d3d11va",1,true) then selection.submit(i); found=true end end
    -- more-menu schedules its action after closing; publication already proves
    -- it uses hwdec-current rather than assuming requested auto means GPU.
    assert(found,"actual decoder missing from overflow menu")
    assert(notice and notice:find("d3d11va",1,true),"decoder details did not show actual driver")
    props["video-format"]=nil; props["hwdec-current"]=nil; observed("video-format")
    assert(props["user-data/javboss/decoder-status"].label=="无视频或等待视频加载")
end
local ok,err=pcall(run)
if ok then real_mp.msg.info("ADAPTIVE_SEEK_TESTS_PASSED") else real_mp.msg.error(tostring(err)) end
real_mp.commandv("quit",ok and 0 or 1)
