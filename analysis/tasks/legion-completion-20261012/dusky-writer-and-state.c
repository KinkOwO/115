// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141c292f0 FUN_141c292f0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c292f0(undefined4 param_1,undefined2 param_2)

{
  char cVar1;
  bool bVar2;
  code *pcVar3;
  undefined8 *puVar4;
  undefined8 uVar5;
  undefined8 *puVar6;
  undefined8 *puVar7;
  undefined8 *puVar8;
  undefined auStack_78 [32];
  undefined8 *puStack_58;
  undefined8 uStack_50;
  undefined8 *puStack_48;
  uint uStack_40;
  undefined4 uStack_3c;
  undefined8 uStack_30;
  undefined auStack_28 [13];
  undefined4 uStack_1b;
  undefined2 uStack_17;
  ulonglong uStack_10;
  
  uStack_30 = 0xfffffffffffffffe;
  uStack_10 = _DAT_14dca9408 ^ (ulonglong)auStack_78;
  cVar1 = *(char *)((longlong)(undefined8 *)_DAT_14e652c80[1] + 0x19);
  puVar6 = (undefined8 *)_DAT_14e652c80[1];
  puVar7 = _DAT_14e652c80;
  while (puVar8 = puVar6, cVar1 == '\0') {
    if (*(int *)((longlong)puVar8 + 0x1c) < 0x85f) {
      puVar6 = (undefined8 *)puVar8[2];
      puVar8 = puVar7;
    }
    else {
      puVar6 = (undefined8 *)*puVar8;
    }
    cVar1 = *(char *)((longlong)puVar6 + 0x19);
    puVar7 = puVar8;
  }
  if (((*(char *)((longlong)puVar7 + 0x19) != '\0') || (0x85f < *(int *)((longlong)puVar7 + 0x1c)))
     || (puVar7 == _DAT_14e652c80)) {
    uStack_1b = param_1;
    uStack_17 = param_2;
    uVar5 = func_0x000146d74000();
    func_0x000146d746e0(uVar5,0x85f);
    uVar5 = func_0x000146d74000();
    func_0x000146d75b10(uVar5,auStack_28,0x13);
    func_0x000146d75af0();
    puVar8 = _DAT_14e652c80;
    puVar6 = (undefined8 *)_DAT_14e652c80[1];
    uStack_40 = 0;
    cVar1 = *(char *)((longlong)puVar6 + 0x19);
    puStack_48 = puVar6;
    puVar7 = _DAT_14e652c80;
    while (puVar4 = puVar6, cVar1 == '\0') {
      bVar2 = 0x85e < *(int *)((longlong)puVar4 + 0x1c);
      if (bVar2) {
        puVar6 = (undefined8 *)*puVar4;
        puVar7 = puVar4;
      }
      else {
        puVar6 = (undefined8 *)puVar4[2];
      }
      uStack_40 = (uint)bVar2;
      cVar1 = *(char *)((longlong)puVar6 + 0x19);
      puStack_48 = puVar4;
    }
    if ((*(char *)((longlong)puVar7 + 0x19) != '\0') || (0x85f < *(int *)((longlong)puVar7 + 0x1c)))
    {
      if (_DAT_14e652c88 == 0x7ffffffffffffff) {
        func_0x00014014f360();
        pcVar3 = (code *)swi(3);
        (*pcVar3)();
        return;
      }
      puStack_58 = (undefined8 *)&DAT_14e652c80;
      uStack_50 = 0;
      puVar6 = (undefined8 *)func_0x000146e8ba20(0x20);
      *(undefined4 *)((longlong)puVar6 + 0x1c) = 0x85f;
      *puVar6 = puVar8;
      puVar6[1] = puVar8;
      puVar6[2] = puVar8;
      *(undefined2 *)(puVar6 + 3) = 0;
      puStack_58 = puStack_48;
      uStack_50 = CONCAT44(uStack_3c,uStack_40);
      func_0x00014014f0e0(&DAT_14e652c80,&puStack_58,puVar6);
    }
  }
  func_0x000148860610(uStack_10 ^ (ulonglong)auStack_78);
  return;
}



// FUNCTION 141c234a0 FUN_141c234a0

void FUN_141c234a0(longlong param_1,undefined4 param_2,undefined4 param_3)

{
  *(undefined4 *)(param_1 + 0x130) = param_2;
  *(undefined4 *)(param_1 + 0x134) = param_3;
  return;
}



// FUNCTION 141c234d0 FUN_141c234d0

void FUN_141c234d0(longlong param_1,undefined4 param_2)

{
  *(undefined4 *)(param_1 + 0x140) = param_2;
  return;
}



