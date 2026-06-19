-- Gran Turismo 4 [SCES-51719] — psdevwiki / PSX-Place
--emu=siren v2

apiRequest(0.1)

local eeObj = getEEObject()
local emuObj = getEmuObject()

local progressive = function()
  eeObj.WriteMem32(0x20A57E70, 0x00000001)
  eeObj.WriteMem32(0x201074A0, 0x24050003)
  eeObj.WriteMem32(0x2061868C, 0x00000001)
  eeObj.WriteMem32(0x20618694, 0x00000000)
  eeObj.WriteMem32(0x20436820, 0xAE0516B0)
  eeObj.WriteMem16(0x20436910, 0x10E8)
end

emuObj.AddVsyncHook(progressive)
