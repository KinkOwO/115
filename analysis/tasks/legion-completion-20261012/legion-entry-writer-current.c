// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 1424fe290 FUN_1424fe290

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_1424fe290(undefined4 param_1,undefined4 param_2)

{
  char cVar1;
  undefined8 *puVar2;
  undefined8 *puVar3;
  undefined8 uVar4;
  undefined8 *puVar5;
  undefined auStack_48 [32];
  undefined auStack_28 [13];
  undefined4 uStack_1b;
  undefined4 uStack_17;
  ulonglong uStack_10;
  
  uStack_10 = _DAT_14dca9408 ^ (ulonglong)auStack_48;
  cVar1 = *(char *)((longlong)(undefined8 *)_DAT_14e652c80[1] + 0x19);
  puVar3 = (undefined8 *)_DAT_14e652c80[1];
  puVar2 = _DAT_14e652c80;
  while (puVar5 = puVar3, cVar1 == '\0') {
    if (*(int *)((longlong)puVar5 + 0x1c) < 0x7fd) {
      puVar3 = (undefined8 *)puVar5[2];
      puVar5 = puVar2;
    }
    else {
      puVar3 = (undefined8 *)*puVar5;
    }
    cVar1 = *(char *)((longlong)puVar3 + 0x19);
    puVar2 = puVar5;
  }
  if (((*(char *)((longlong)puVar2 + 0x19) != '\0') || (0x7fd < *(int *)((longlong)puVar2 + 0x1c)))
     || (puVar2 == _DAT_14e652c80)) {
    uStack_1b = param_1;
    uStack_17 = param_2;
    uVar4 = func_0x000146d74000();
    func_0x000146d746e0(uVar4,0x7fd);
    uVar4 = func_0x000146d74000();
    func_0x000146d75b10(uVar4,auStack_28,0x15);
    func_0x000146d75af0();
    func_0x000140dd8720(0x7fd);
  }
  func_0x000148860610(uStack_10 ^ (ulonglong)auStack_48);
  return;
}


