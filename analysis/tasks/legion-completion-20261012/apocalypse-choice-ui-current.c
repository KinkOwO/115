// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 1406a1fa0 FUN_1406a1fa0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_1406a1fa0(longlong param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  longlong *plVar1;
  longlong lVar2;
  undefined8 unaff_RBX;
  undefined8 uVar3;
  
  uVar3 = 0xfffffffffffffffe;
  if (_DAT_14e638998 == 0) {
    lVar2 = func_0x000146e8ba20(0x158);
    if (lVar2 != 0) {
      lVar2 = FUN_140697290(lVar2);
    }
    _DAT_14e638998 = lVar2;
    (***(code ***)(lVar2 + 0x50))();
  }
  FUN_14069b560(_DAT_14e638998,*(undefined4 *)(*(longlong *)(param_1 + 8) + 0x3ec),param_3,param_4,
                uVar3);
  plVar1 = *(longlong **)(*(longlong *)(param_1 + 8) + 0x3b0);
  (**(code **)(*plVar1 + 0x10))(plVar1,0,param_3,param_4,unaff_RBX);
                    /* WARNING: Could not recover jumptable at 0x000144a8eace. Too many branches */
                    /* WARNING: Treating indirect jump as call */
  (**(code **)(*plVar1 + 0x18))(plVar1,0);
  return;
}



// FUNCTION 1406af0d0 FUN_1406af0d0

undefined8 FUN_1406af0d0(longlong param_1)

{
  char cVar1;
  
  if ((((*(longlong *)(param_1 + 0x5f0) != 0) && (*(longlong *)(param_1 + 0x600) != 0)) &&
      (*(longlong *)(param_1 + 0x610) != 0)) && (*(longlong **)(param_1 + 0x680) != (longlong *)0x0)
     ) {
    cVar1 = (**(code **)(**(longlong **)(param_1 + 0x680) + 0x298))();
    if ((cVar1 != '\0') && (*(longlong **)(param_1 + 0x688) != (longlong *)0x0)) {
      cVar1 = (**(code **)(**(longlong **)(param_1 + 0x688) + 0x298))();
      if ((cVar1 != '\0') && (*(longlong **)(param_1 + 0x690) != (longlong *)0x0)) {
        cVar1 = (**(code **)(**(longlong **)(param_1 + 0x690) + 0x298))();
        if ((cVar1 != '\0') && (*(longlong **)(param_1 + 0x678) != (longlong *)0x0)) {
          cVar1 = (**(code **)(**(longlong **)(param_1 + 0x678) + 0x298))();
          if (cVar1 != '\0') {
            return 1;
          }
        }
      }
    }
  }
  return 0;
}



// FUNCTION 140697290 FUN_140697290

undefined8 * FUN_140697290(undefined8 *param_1)

{
  longlong *plVar1;
  longlong lVar2;
  undefined8 uVar3;
  longlong lVar4;
  
  func_0x000146ebb4e0();
  *param_1 = &UNK_1491ef558;
  uVar3 = 0;
  param_1[8] = 0;
  param_1[9] = 0;
  lVar2 = func_0x000146e8ba20(0x48);
  *(longlong *)lVar2 = lVar2;
  *(longlong *)(lVar2 + 8) = lVar2;
  *(longlong *)(lVar2 + 0x10) = lVar2;
  *(undefined2 *)(lVar2 + 0x18) = 0x101;
  param_1[8] = lVar2;
  *param_1 = &UNK_1492e1950;
  param_1[10] = &UNK_1492e1bb8;
  *(undefined2 *)(param_1 + 0xd) = 0xffff;
  *(undefined *)((longlong)param_1 + 0x6a) = 0xff;
  *(undefined4 *)((longlong)param_1 + 0x6b) = 0xe;
  *(undefined4 *)((longlong)param_1 + 0x6f) = 4;
  *(undefined8 *)((longlong)param_1 + 0x73) = 0xffffffffffffffff;
  *(undefined4 *)((longlong)param_1 + 0x7f) = 0xffffffff;
  *(undefined *)((longlong)param_1 + 0x7b) = 0;
  *(undefined2 *)((longlong)param_1 + 0x7d) = 0;
  lVar2 = (longlong)param_1 + 0x83;
  lVar4 = 6;
  do {
    func_0x0001471b0f60(lVar2);
    lVar2 = lVar2 + 0xc;
    lVar4 = lVar4 + -1;
  } while (lVar4 != 0);
  *(undefined4 *)((longlong)param_1 + 0xcb) = 0xffffffff;
  *(undefined4 *)((longlong)param_1 + 0xcf) = 0;
  func_0x000140698280(param_1 + 0xd);
  plVar1 = param_1 + 0x27;
  *plVar1 = 0;
  param_1[0x28] = 0;
  lVar2 = func_0x000146e8ba20(0x30);
  *(longlong *)lVar2 = lVar2;
  *(longlong *)(lVar2 + 8) = lVar2;
  *(longlong *)(lVar2 + 0x10) = lVar2;
  *(undefined2 *)(lVar2 + 0x18) = 0x101;
  *plVar1 = lVar2;
  *(undefined4 *)((longlong)param_1 + 0x154) = 0;
  param_1[0xb] = 0;
  param_1[0xc] = 0;
  lVar2 = *plVar1;
  func_0x00014014ebd0(plVar1,plVar1,*(undefined8 *)(lVar2 + 8));
  *(longlong *)(lVar2 + 8) = lVar2;
  *(longlong *)lVar2 = lVar2;
  *(longlong *)(lVar2 + 0x10) = lVar2;
  param_1[0x28] = 0;
  *(undefined2 *)(param_1 + 0x29) = 0;
  *(undefined4 *)((longlong)param_1 + 0x14c) = 0xffffffff;
  *(undefined *)(param_1 + 0x2a) = 0;
  lVar2 = func_0x000146e8ba20(0x188);
  if (lVar2 != 0) {
    uVar3 = FUN_140695cd0(lVar2,0x6b);
  }
  param_1[0xb] = uVar3;
  return param_1;
}



// FUNCTION 14069b560 FUN_14069b560

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14069b560(longlong param_1,undefined param_2)

{
  char cVar1;
  undefined8 *puVar2;
  undefined8 *puVar3;
  undefined8 uVar4;
  undefined8 *puVar5;
  undefined auStack_48 [32];
  undefined auStack_28 [13];
  undefined4 uStack_1b;
  undefined uStack_17;
  undefined4 uStack_16;
  ulonglong uStack_10;
  
  if (*(char *)(param_1 + 0x148) == '\0') {
    return;
  }
  uStack_10 = _DAT_14dca9408 ^ (ulonglong)auStack_48;
  cVar1 = *(char *)((longlong)(undefined8 *)_DAT_14e652c80[1] + 0x19);
  puVar3 = (undefined8 *)_DAT_14e652c80[1];
  puVar2 = _DAT_14e652c80;
  while (puVar5 = puVar3, cVar1 == '\0') {
    if (*(int *)((longlong)puVar5 + 0x1c) < 0x932) {
      puVar3 = (undefined8 *)puVar5[2];
      puVar5 = puVar2;
    }
    else {
      puVar3 = (undefined8 *)*puVar5;
    }
    cVar1 = *(char *)((longlong)puVar3 + 0x19);
    puVar2 = puVar5;
  }
  if (((*(char *)((longlong)puVar2 + 0x19) != '\0') || (0x932 < *(int *)((longlong)puVar2 + 0x1c)))
     || (puVar2 == _DAT_14e652c80)) {
    uStack_16 = 0x6b;
    uStack_1b = 2;
    uStack_17 = param_2;
    uVar4 = func_0x000146d74000();
    func_0x000146d746e0(uVar4,0x932);
    uVar4 = func_0x000146d74000();
    func_0x000146d75b10(uVar4,auStack_28,0x16);
    func_0x000146d75af0();
    func_0x000140dd8720(0x932);
  }
  func_0x000148860610(uStack_10 ^ (ulonglong)auStack_48);
  return;
}


