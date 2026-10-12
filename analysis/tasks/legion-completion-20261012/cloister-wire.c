// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 1420a5c50 FUN_1420a5c50

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_1420a5c50(undefined4 param_1,undefined2 param_2)

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
    if (*(int *)((longlong)puVar8 + 0x1c) < 0x820) {
      puVar6 = (undefined8 *)puVar8[2];
      puVar8 = puVar7;
    }
    else {
      puVar6 = (undefined8 *)*puVar8;
    }
    cVar1 = *(char *)((longlong)puVar6 + 0x19);
    puVar7 = puVar8;
  }
  if (((*(char *)((longlong)puVar7 + 0x19) != '\0') || (0x820 < *(int *)((longlong)puVar7 + 0x1c)))
     || (puVar7 == _DAT_14e652c80)) {
    uStack_1b = param_1;
    uStack_17 = param_2;
    uVar5 = func_0x000146d74000();
    func_0x000146d746e0(uVar5,0x820);
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
      bVar2 = 0x81f < *(int *)((longlong)puVar4 + 0x1c);
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
    if ((*(char *)((longlong)puVar7 + 0x19) != '\0') || (0x820 < *(int *)((longlong)puVar7 + 0x1c)))
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
      *(undefined4 *)((longlong)puVar6 + 0x1c) = 0x820;
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



// FUNCTION 1420a5730 FUN_1420a5730

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_1420a5730(void)

{
  longlong *plVar1;
  int *piVar2;
  int iVar3;
  undefined *puVar4;
  undefined8 *puVar5;
  longlong lVar6;
  longlong lVar7;
  longlong lVar8;
  undefined8 uVar9;
  longlong lVar10;
  undefined auStack_c8 [32];
  longlong lStack_a8;
  undefined8 uStack_a0;
  undefined auStack_98 [8];
  longlong *plStack_90;
  undefined2 uStack_88;
  undefined uStack_86;
  int iStack_85;
  undefined4 uStack_81;
  ulonglong uStack_7d;
  undefined4 uStack_75;
  undefined8 auStack_71 [5];
  undefined auStack_45 [36];
  undefined uStack_21;
  undefined2 uStack_1f;
  ulonglong uStack_18;
  
  uStack_a0 = 0xfffffffffffffffe;
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_c8;
  uStack_88 = 0xffff;
  uStack_86 = 0xff;
  iStack_85 = 0xe;
  lVar10 = 4;
  uStack_81 = 4;
  uStack_7d = 0xffffffffffffffff;
  uStack_75 = 0xffffffff;
  puVar5 = auStack_71;
  lVar8 = 5;
  lVar6 = 5;
  do {
    *puVar5 = 0xffffffffffffffff;
    puVar5 = puVar5 + 1;
    lVar6 = lVar6 + -1;
  } while (lVar6 != 0);
  lVar7 = 4;
  puVar4 = auStack_45;
  lVar6 = 0;
  do {
    *(undefined8 *)(puVar4 + -4) = 0;
    puVar4[4] = 0;
    puVar4 = puVar4 + 10;
    lVar7 = lVar7 + -1;
  } while (lVar7 != 0);
  puVar5 = auStack_71;
  do {
    *puVar5 = 0xffffffffffffffff;
    puVar5 = puVar5 + 1;
    lVar8 = lVar8 + -1;
  } while (lVar8 != 0);
  puVar4 = auStack_45;
  do {
    *(undefined8 *)(puVar4 + -4) = 0;
    puVar4[4] = 0;
    puVar4 = puVar4 + 10;
    lVar10 = lVar10 + -1;
  } while (lVar10 != 0);
  uStack_21 = 0;
  uStack_1f = 0;
  func_0x000146ea0be0(&uStack_88,0x6b);
  if (_DAT_14e650e88 == 0) {
    lStack_a8 = func_0x000146e8ba20(0x128);
    lVar8 = lVar6;
    if (lStack_a8 != 0) {
      lVar8 = func_0x00014209af10(lStack_a8);
    }
    _DAT_14e650e88 = lVar8;
    (***(code ***)(lVar8 + 0x50))();
  }
  FUN_14209e590(_DAT_14e650e88,&uStack_88);
  func_0x000142ac2220(_DAT_14e683c40,uStack_81);
  func_0x000142ac2290(_DAT_14e683c40,iStack_85);
  func_0x000142ac21b0(_DAT_14e683c40,uStack_7d & 0xffffffff);
  if (iStack_85 == 6) {
    if (_DAT_14e650e88 == 0) {
      lStack_a8 = func_0x000146e8ba20(0x128);
      lVar8 = lVar6;
      if (lStack_a8 != 0) {
        lVar8 = func_0x00014209af10(lStack_a8);
      }
      _DAT_14e650e88 = lVar8;
      (***(code ***)(lVar8 + 0x50))();
    }
    uVar9 = 0;
    lVar8 = _DAT_14e650e88;
  }
  else {
    if (iStack_85 != 2) goto LAB_1420a58f4;
    lVar8 = func_0x000142080c60();
    uVar9 = 2;
  }
  func_0x00014106e400(lVar8,uVar9);
LAB_1420a58f4:
  if (_DAT_14e650e88 == 0) {
    lStack_a8 = func_0x000146e8ba20(0x128);
    if (lStack_a8 != 0) {
      lVar6 = func_0x00014209af10(lStack_a8);
    }
    _DAT_14e650e88 = lVar6;
    (***(code ***)(lVar6 + 0x50))();
  }
  puVar5 = (undefined8 *)func_0x0001414c56e0(_DAT_14e650e88,auStack_98);
  func_0x000142099e20(*puVar5);
  if (plStack_90 != (longlong *)0x0) {
    LOCK();
    plVar1 = plStack_90 + 1;
    iVar3 = *(int *)plVar1;
    *(int *)plVar1 = *(int *)plVar1 + -1;
    UNLOCK();
    if (iVar3 == 1) {
      (**(code **)*plStack_90)(plStack_90);
      LOCK();
      piVar2 = (int *)((longlong)plStack_90 + 0xc);
      iVar3 = *piVar2;
      *piVar2 = *piVar2 + -1;
      UNLOCK();
      if (iVar3 == 1) {
        (**(code **)(*plStack_90 + 8))(plStack_90);
      }
    }
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_c8);
  return;
}



// FUNCTION 14209e590 FUN_14209e590

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14209e590(longlong param_1,undefined2 *param_2)

{
  undefined8 uVar1;
  int iVar2;
  longlong lVar3;
  undefined8 uVar4;
  undefined8 uVar5;
  undefined *puStack_50;
  longlong lStack_48;
  undefined **ppuStack_18;
  
  *(undefined2 *)(param_1 + 0x60) = *param_2;
  *(undefined *)(param_1 + 0x62) = *(undefined *)(param_2 + 1);
  *(undefined4 *)(param_1 + 99) = *(undefined4 *)((longlong)param_2 + 3);
  *(undefined4 *)(param_1 + 0x67) = *(undefined4 *)((longlong)param_2 + 7);
  *(undefined4 *)(param_1 + 0x6b) = *(undefined4 *)((longlong)param_2 + 0xb);
  *(undefined4 *)(param_1 + 0x6f) = *(undefined4 *)((longlong)param_2 + 0xf);
  *(undefined4 *)(param_1 + 0x73) = *(undefined4 *)((longlong)param_2 + 0x13);
  *(undefined8 *)(param_1 + 0x77) = *(undefined8 *)((longlong)param_2 + 0x17);
  *(undefined8 *)(param_1 + 0x7f) = *(undefined8 *)((longlong)param_2 + 0x1f);
  *(undefined8 *)(param_1 + 0x87) = *(undefined8 *)((longlong)param_2 + 0x27);
  *(undefined8 *)(param_1 + 0x8f) = *(undefined8 *)((longlong)param_2 + 0x2f);
  *(undefined8 *)(param_1 + 0x97) = *(undefined8 *)((longlong)param_2 + 0x37);
  *(undefined8 *)(param_1 + 0x9f) = *(undefined8 *)((longlong)param_2 + 0x3f);
  *(undefined2 *)(param_1 + 0xa7) = *(undefined2 *)((longlong)param_2 + 0x47);
  *(undefined8 *)(param_1 + 0xa9) = *(undefined8 *)((longlong)param_2 + 0x49);
  *(undefined2 *)(param_1 + 0xb1) = *(undefined2 *)((longlong)param_2 + 0x51);
  *(undefined8 *)(param_1 + 0xb3) = *(undefined8 *)((longlong)param_2 + 0x53);
  *(undefined2 *)(param_1 + 0xbb) = *(undefined2 *)((longlong)param_2 + 0x5b);
  *(undefined8 *)(param_1 + 0xbd) = *(undefined8 *)((longlong)param_2 + 0x5d);
  *(undefined2 *)(param_1 + 0xc5) = *(undefined2 *)((longlong)param_2 + 0x65);
  *(undefined *)(param_1 + 199) = *(undefined *)((longlong)param_2 + 0x67);
  *(undefined2 *)(param_1 + 0xc9) = *(undefined2 *)((longlong)param_2 + 0x69);
  if (*(char *)(param_1 + 0x62) != -1) {
    if (*(char *)(param_1 + 0x58) == '\x01') {
      return;
    }
    if (((*(int *)((longlong)param_2 + 3) == 2) &&
        (iVar2 = func_0x0001459a90f0(_DAT_14e66c090), iVar2 == 1)) && (*(byte *)(param_2 + 1) < 4))
    {
      func_0x00014209fbe0(param_1,(int)*(short *)((longlong)param_2 +
                                                 (longlong)(char)*(byte *)(param_2 + 1) * 10 + 0x3f)
                         );
    }
  }
  if (*(int *)(param_1 + 99) == 1) {
    uVar1 = *(undefined8 *)(param_1 + 0x118);
    lVar3 = func_0x000146e8ba20(0x20);
    uVar5 = 0;
    uVar4 = uVar5;
    if (lVar3 != 0) {
      uVar4 = func_0x00014338a980(lVar3,3000,0);
    }
    func_0x00014338f710(uVar1,uVar4,0);
    uVar1 = *(undefined8 *)(param_1 + 0x118);
    lVar3 = func_0x000146e8ba20(0x50);
    if (lVar3 != 0) {
      puStack_50 = &UNK_1498fa848;
      ppuStack_18 = &puStack_50;
      lStack_48 = param_1;
      uVar5 = func_0x00014338c160(lVar3,&puStack_50);
    }
    func_0x00014338f710(uVar1,uVar5,0);
  }
  func_0x0001420a07e0(param_1);
  return;
}


