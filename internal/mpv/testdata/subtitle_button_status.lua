local real_mp = require "mp"
local source_file = assert(io.open(real_mp.get_opt("test-script"), "r"))
local source = source_file:read("*a")
source_file:close()
local helper = assert(source:match("(local function subtitle_button_status%(%)%s.-)local function set_osc_locale"))
local properties = {}
local get_status = assert(loadstring("return function(mp)\n" .. helper .. "\nreturn subtitle_button_status end"))()({
    get_property_native = function(key, fallback)
        if properties[key] ~= nil then return properties[key] end
        return fallback
    end,
})
local function check(status, selected, tracks, label, color)
    properties["user-data/javboss/subtitle-status"] = status
    properties["user-data/javboss/subtitle-menu-items"] = selected and {{selected=true,label="test.srt（下载）"}} or {}
    properties["track-list"] = tracks and {{type="sub",id=1}} or {}
    local actual, tint, detail = get_status()
    assert(actual == label, actual .. " ~= " .. label)
    assert(tint == color, tint .. " ~= " .. color)
    assert(type(detail) == "string" and #detail > 0)
end
local ok, err = pcall(function()
    check("",false,false,"字幕","CDD0D7")
    check("searching",false,false,"字幕 查找中","FF840A")
    check("ready",true,true,"字幕 已启用","58D130")
    check("existing",true,true,"字幕 已启用","58D130")
    check("ready",false,true,"字幕 已关闭","CDD0D7")
    check("existing",false,true,"字幕 已关闭","CDD0D7")
    check("not_found",false,false,"字幕 未找到","0A9FFF")
    check("ambiguous",false,false,"字幕 无匹配","0A9FFF")
    check("error",false,false,"字幕 失败","3A45FF")
    check("disabled",false,false,"字幕 手动","CDD0D7")
    check("error",true,true,"字幕 已启用","58D130")
    local _, _, detail = get_status()
    assert(detail:find("在线查找失败",1,true))
    check("searching",true,true,"字幕 查找中","FF840A")
    check("",false,false,"字幕","CDD0D7") -- next video cleared the prior success
end)
if ok then real_mp.msg.info("SUBTITLE_BUTTON_STATUS_TESTS_PASSED") else real_mp.msg.error(tostring(err)) end
real_mp.commandv("quit",ok and 0 or 1)
