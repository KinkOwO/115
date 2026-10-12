// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141c0f680 FUN_141c0f680

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c0f680(undefined8 *param_1,undefined8 param_2,int param_3)

{
  int *piVar1;
  longlong *plVar2;
  undefined8 *puVar3;
  code *pcVar4;
  undefined8 uVar5;
  longlong lVar6;
  longlong lVar7;
  longlong lVar8;
  ulonglong uVar9;
  undefined auStack_b8 [32];
  undefined8 uStack_98;
  undefined auStack_88 [4];
  uint uStack_84;
  undefined8 *puStack_80;
  undefined8 uStack_78;
  undefined8 *puStack_70;
  undefined8 *puStack_68;
  undefined auStack_60 [32];
  undefined8 *puStack_40;
  undefined8 *puStack_38;
  longlong *plStack_30;
  ulonglong uStack_28;
  ulonglong uStack_20;
  
  uStack_78 = 0xfffffffffffffffe;
  uStack_20 = _DAT_14dca9408 ^ (ulonglong)auStack_b8;
  lVar8 = 0;
  uStack_84 = 0;
  puStack_70 = param_1;
  if (param_3 != 0) {
    param_1[4] = &UNK_1497e7f08;
    puStack_80 = param_1 + 0x1ba;
    func_0x000146ebb4e0();
  }
  uStack_84 = (uint)(param_3 != 0);
  puStack_80 = (undefined8 *)auStack_60;
  uVar5 = func_0x000146e8c7d0(&UNK_1497e8300);
  uVar5 = func_0x00014014c7b0(auStack_60,uVar5);
  uStack_98 = (undefined *)((ulonglong)uStack_98._4_4_ << 0x20);
  func_0x000145f6da20(param_1,param_2,0x289,uVar5);
  *param_1 = &UNK_1497e7b08;
  param_1[3] = &UNK_1497e7ed0;
  param_1[0xa7] = &UNK_1497e7f00;
  func_0x000141c0fe30(param_1 + 0xbd);
  func_0x000141c0fc30(param_1 + 0x13e);
  param_1[0x196] = 0;
  param_1[0x197] = 0;
  param_1[0x198] = 0;
  param_1[0x199] = 0;
  param_1[0x19a] = 0;
  param_1[0x19b] = 0;
  puStack_68 = param_1 + 0x19c;
  *puStack_68 = 0;
  param_1[0x19d] = 0;
  param_1[0x19e] = 0;
  param_1[0x19f] = 0;
  param_1[0x1a0] = 0;
  param_1[0x1a1] = 0;
  param_1[0x1a2] = 0;
  param_1[0x1a3] = 0;
  param_1[0x1a4] = 0;
  param_1[0x1a5] = 0;
  param_1[0x1a6] = 0;
  param_1[0x1a7] = 0;
  uStack_98 = &SUB_1401566d0;
  func_0x000148860630(param_1 + 0x1a9,0x10,3,&SUB_1401eb0d0);
  uStack_98 = &SUB_1401566d0;
  func_0x000148860630(param_1 + 0x1af,0x10,3,&SUB_1401eb0d0);
  plVar2 = param_1 + 0x1b6;
  *plVar2 = 0;
  param_1[0x1b7] = 0;
  param_1[0x1b8] = 0;
  func_0x000141c15570(param_1);
  puVar3 = (undefined8 *)param_1[0x1b7];
  lVar6 = *plVar2;
  lVar7 = ((longlong)puVar3 - lVar6) / 6 + ((longlong)puVar3 - lVar6 >> 0x3f);
  uVar9 = (lVar7 >> 2) - (lVar7 >> 0x3f);
  if (uVar9 < 6) {
    if (uVar9 < 5) {
      if ((ulonglong)((param_1[0x1b8] - lVar6) / 0x18) < 5) {
        func_0x000141c0f2e0(plVar2,5,auStack_88);
      }
      else {
        puStack_40 = puVar3;
        for (lVar6 = 5 - uVar9; lVar6 != 0; lVar6 = lVar6 + -1) {
          *puStack_40 = 0;
          puStack_40[1] = 0;
          puStack_40[2] = 0;
          puStack_40 = puStack_40 + 3;
        }
        param_1[0x1b7] = puStack_40;
        puStack_38 = puStack_40;
        plStack_30 = plVar2;
      }
    }
  }
  else {
    func_0x000141c0f230(lVar6 + 0x78,puVar3,plVar2);
    param_1[0x1b7] = lVar6 + 0x78;
  }
  lVar6 = param_1[0x1b6];
  puVar3 = *(undefined8 **)(lVar6 + 8);
  if (puVar3 == *(undefined8 **)(lVar6 + 0x10)) {
    func_0x0001401e7aa0();
  }
  else {
    *puVar3 = 0;
    puVar3[1] = 0;
    if (param_1[0x182] != 0) {
      LOCK();
      piVar1 = (int *)(param_1[0x182] + 8);
      *piVar1 = *piVar1 + 1;
      UNLOCK();
    }
    *puVar3 = param_1[0x181];
    puVar3[1] = param_1[0x182];
    *(longlong *)(lVar6 + 8) = *(longlong *)(lVar6 + 8) + 0x10;
  }
  lVar6 = param_1[0x1b6];
  puVar3 = *(undefined8 **)(lVar6 + 0x38);
  if (puVar3 == *(undefined8 **)(lVar6 + 0x40)) {
    func_0x0001401e7aa0();
  }
  else {
    *puVar3 = 0;
    puVar3[1] = 0;
    if (param_1[0x136] != 0) {
      LOCK();
      piVar1 = (int *)(param_1[0x136] + 8);
      *piVar1 = *piVar1 + 1;
      UNLOCK();
    }
    *puVar3 = param_1[0x135];
    puVar3[1] = param_1[0x136];
    *(longlong *)(lVar6 + 0x38) = *(longlong *)(lVar6 + 0x38) + 0x10;
  }
  lVar6 = param_1[0x1b6];
  puVar3 = *(undefined8 **)(lVar6 + 0x38);
  if (puVar3 == *(undefined8 **)(lVar6 + 0x40)) {
    func_0x0001401e7aa0();
  }
  else {
    *puVar3 = 0;
    puVar3[1] = 0;
    if (param_1[0x138] != 0) {
      LOCK();
      piVar1 = (int *)(param_1[0x138] + 8);
      *piVar1 = *piVar1 + 1;
      UNLOCK();
    }
    *puVar3 = param_1[0x137];
    puVar3[1] = param_1[0x138];
    *(longlong *)(lVar6 + 0x38) = *(longlong *)(lVar6 + 0x38) + 0x10;
  }
  lVar6 = param_1[0x1b6];
  puVar3 = *(undefined8 **)(lVar6 + 0x50);
  if (puVar3 == *(undefined8 **)(lVar6 + 0x58)) {
    func_0x0001401e7aa0();
  }
  else {
    *puVar3 = 0;
    puVar3[1] = 0;
    if (param_1[0x186] != 0) {
      LOCK();
      piVar1 = (int *)(param_1[0x186] + 8);
      *piVar1 = *piVar1 + 1;
      UNLOCK();
    }
    *puVar3 = param_1[0x185];
    puVar3[1] = param_1[0x186];
    *(longlong *)(lVar6 + 0x50) = *(longlong *)(lVar6 + 0x50) + 0x10;
  }
  lVar6 = param_1[0x1b6];
  puVar3 = *(undefined8 **)(lVar6 + 0x50);
  if (puVar3 == *(undefined8 **)(lVar6 + 0x58)) {
    func_0x0001401e7aa0();
  }
  else {
    *puVar3 = 0;
    puVar3[1] = 0;
    if (param_1[0x18a] != 0) {
      LOCK();
      piVar1 = (int *)(param_1[0x18a] + 8);
      *piVar1 = *piVar1 + 1;
      UNLOCK();
    }
    *puVar3 = param_1[0x189];
    puVar3[1] = param_1[0x18a];
    *(longlong *)(lVar6 + 0x50) = *(longlong *)(lVar6 + 0x50) + 0x10;
  }
  lVar6 = param_1[0x1b6];
  puVar3 = *(undefined8 **)(lVar6 + 0x68);
  if (puVar3 == *(undefined8 **)(lVar6 + 0x70)) {
    func_0x0001401e7aa0();
  }
  else {
    *puVar3 = 0;
    puVar3[1] = 0;
    if (param_1[400] != 0) {
      LOCK();
      piVar1 = (int *)(param_1[400] + 8);
      *piVar1 = *piVar1 + 1;
      UNLOCK();
    }
    *puVar3 = param_1[399];
    puVar3[1] = param_1[400];
    *(longlong *)(lVar6 + 0x68) = *(longlong *)(lVar6 + 0x68) + 0x10;
  }
  lVar6 = param_1[0x1b6];
  puVar3 = *(undefined8 **)(lVar6 + 0x68);
  if (puVar3 == *(undefined8 **)(lVar6 + 0x70)) {
    func_0x0001401e7aa0();
  }
  else {
    *puVar3 = 0;
    puVar3[1] = 0;
    if (param_1[0x192] != 0) {
      LOCK();
      piVar1 = (int *)(param_1[0x192] + 8);
      *piVar1 = *piVar1 + 1;
      UNLOCK();
    }
    *puVar3 = param_1[0x191];
    puVar3[1] = param_1[0x192];
    *(longlong *)(lVar6 + 0x68) = *(longlong *)(lVar6 + 0x68) + 0x10;
  }
  do {
    lVar6 = param_1[0x1b6];
    puVar3 = *(undefined8 **)(lVar6 + 0x20);
    if (puVar3 == *(undefined8 **)(lVar6 + 0x28)) {
      func_0x0001401e7aa0();
    }
    else {
      *puVar3 = 0;
      puVar3[1] = 0;
      lVar7 = *(longlong *)((longlong)param_1 + lVar8 + 0x6f0);
      if (lVar7 != 0) {
        LOCK();
        piVar1 = (int *)(lVar7 + 8);
        *piVar1 = *piVar1 + 1;
        UNLOCK();
      }
      *puVar3 = *(undefined8 *)((longlong)param_1 + lVar8 + 0x6e8);
      puVar3[1] = *(undefined8 *)((longlong)param_1 + lVar8 + 0x6f0);
      *(longlong *)(lVar6 + 0x20) = *(longlong *)(lVar6 + 0x20) + 0x10;
    }
    lVar8 = lVar8 + 0xb8;
  } while (lVar8 < 0x2e0);
  *(undefined4 *)(param_1 + 0x1b9) = 0xffffffff;
  uVar5 = func_0x000146e8c7d0(&UNK_1492cce10);
  func_0x00014014c7b0(&puStack_40,uVar5);
  func_0x0001467aa190(param_1,&puStack_40);
  if (7 < uStack_28) {
    if (0xfff < uStack_28 * 2 + 2) {
      if (0x1f < (ulonglong)((longlong)puStack_40 + (-8 - puStack_40[-1]))) {
        func_0x000148aaf304(puStack_40[-1],uStack_28 * 2 + 0x29);
        pcVar4 = (code *)swi(3);
        (*pcVar4)();
        return;
      }
    }
    func_0x000146e9f3a0();
  }
  plStack_30 = (longlong *)0x0;
  uStack_28 = 7;
  puStack_40 = (undefined8 *)((ulonglong)puStack_40 & 0xffffffffffff0000);
  func_0x000148860610(uStack_20 ^ (ulonglong)auStack_b8);
  return;
}



// FUNCTION 141c18aa0 FUN_141c18aa0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

undefined8 FUN_141c18aa0(longlong param_1)

{
  char cVar1;
  uint uVar2;
  longlong lVar3;
  undefined8 uVar4;
  longlong *plVar5;
  
  cVar1 = func_0x000145f6f4b0();
  if (cVar1 != '\0') {
    *(undefined4 *)(param_1 + 0x9e8) = 3;
    *(undefined4 *)(param_1 + 0xda8) = 0xffffffff;
    *(undefined4 *)(param_1 + 0xdcc) = 0xffffffff;
    func_0x0001467a8b30(param_1);
    func_0x0001467a8a70(param_1);
    if (_DAT_14e64f730 == 0) {
      lVar3 = func_0x000146e8ba20(0x170);
      if (lVar3 != 0) {
        lVar3 = FUN_141c1fad0(lVar3);
      }
      _DAT_14e64f730 = lVar3;
      (***(code ***)(lVar3 + 0x50))();
    }
    uVar2 = func_0x000141c21460(_DAT_14e64f730);
    *(uint *)(param_1 + 0xda8) = uVar2;
    if (uVar2 < 3) {
      func_0x00014338d360(*(undefined8 *)(param_1 + 0x9d0));
      func_0x00014338d360(*(undefined8 *)(param_1 + 0x9e0));
      func_0x00014338d360(*(undefined8 *)(param_1 + 0x9d8));
      func_0x000141c1a3b0(param_1 + 0x9f0,0);
      func_0x000141c1a540(param_1 + 0x5e8,0);
      func_0x000146695380(_DAT_14e683c78,0);
      func_0x00014666c130(_DAT_14e683c78,0xb3b);
      (**(code **)(**(longlong **)(param_1 + 0xd0) + 0x70))
                (*(longlong **)(param_1 + 0xd0),(float)(_DAT_14dc6d8f0 + -0x42b) * _DAT_1491ac5f4,
                 (float)(_DAT_14dc6d8f4 + -600) * _DAT_1491ac5f4);
      func_0x000146eeca40(*(undefined8 *)(param_1 + 0xcb0));
      func_0x000146ecc310(*(undefined8 *)(param_1 + 0xcb0));
      func_0x000146eeca40(*(undefined8 *)(param_1 + 0xcd0));
      func_0x000146ecc310(*(undefined8 *)(param_1 + 0xcd0));
      func_0x000141c1de10(param_1);
      func_0x0001467a9f20(param_1,0);
      uVar4 = func_0x000146e8c7d0(&UNK_1497e8470);
      lVar3 = func_0x000145a0f110(uVar4,0x187);
      if (lVar3 != 0) {
        uVar4 = func_0x000146e8c7d0(&UNK_1497e8470);
        plVar5 = (longlong *)func_0x000145a0f110(uVar4,0x189);
        (**(code **)(*plVar5 + 0x88))(plVar5,0,0,1);
      }
      uVar4 = func_0x000146e8c7d0(&UNK_1497e84c0);
      func_0x000145a31380(uVar4,0xffffffff,0,0,0xffffffff,0xffffffff,0);
      *(undefined4 *)(param_1 + 0xdcc) = 0xffffffff;
      return 1;
    }
  }
  return 0;
}



// FUNCTION 141c18250 FUN_141c18250

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c18250(undefined8 param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  undefined8 uVar1;
  longlong lVar2;
  longlong *plVar3;
  
  func_0x000146695380(_DAT_14e683c78,1,param_3,param_4,0xfffffffffffffffe);
  uVar1 = func_0x000146e8c7d0(&UNK_1497e84f0);
  lVar2 = func_0x000145a0f110(uVar1,0x199);
  if (lVar2 != 0) {
    uVar1 = func_0x000146e8c7d0(&UNK_1497e84f0);
    plVar3 = (longlong *)func_0x000145a0f110(uVar1,0x19b);
    (**(code **)(*plVar3 + 0x88))(plVar3,0,1);
  }
  if (_DAT_14e64f730 == 0) {
    lVar2 = func_0x000146e8ba20(0x170);
    if (lVar2 != 0) {
      lVar2 = FUN_141c1fad0(lVar2);
    }
    _DAT_14e64f730 = lVar2;
    (***(code ***)(lVar2 + 0x50))();
  }
  *(undefined4 *)(_DAT_14e64f730 + 0x134) = 4;
  return;
}



// FUNCTION 141c1fad0 FUN_141c1fad0

undefined8 * FUN_141c1fad0(undefined8 *param_1)

{
  longlong lVar1;
  undefined8 *puVar2;
  undefined8 uVar3;
  longlong lVar4;
  undefined8 *puVar5;
  longlong lVar6;
  undefined8 *puVar7;
  longlong lVar8;
  
  func_0x000146ebb4e0();
  *param_1 = &UNK_1491ef558;
  uVar3 = 0;
  param_1[8] = 0;
  param_1[9] = 0;
  lVar1 = func_0x000146e8ba20(0x48);
  *(longlong *)lVar1 = lVar1;
  *(longlong *)(lVar1 + 8) = lVar1;
  *(longlong *)(lVar1 + 0x10) = lVar1;
  *(undefined2 *)(lVar1 + 0x18) = 0x101;
  param_1[8] = lVar1;
  *param_1 = &UNK_1497e9fe8;
  param_1[10] = &UNK_1497ea250;
  param_1[0xb] = 0;
  param_1[0xc] = 0;
  *(undefined2 *)(param_1 + 0xd) = 0xffff;
  *(undefined *)((longlong)param_1 + 0x6a) = 0xff;
  *(undefined4 *)((longlong)param_1 + 0x6b) = 0xe;
  *(undefined4 *)((longlong)param_1 + 0x6f) = 4;
  *(undefined8 *)((longlong)param_1 + 0x73) = 0xffffffffffffffff;
  *(undefined4 *)((longlong)param_1 + 0x7b) = 0xffffffff;
  lVar1 = (longlong)param_1 + 0x7f;
  lVar8 = 3;
  lVar6 = 3;
  lVar4 = lVar1;
  do {
    func_0x0001471b0f60(lVar4);
    lVar4 = lVar4 + 0xc;
    lVar6 = lVar6 + -1;
  } while (lVar6 != 0);
  puVar7 = (undefined8 *)((longlong)param_1 + 0xa3);
  lVar4 = 3;
  puVar2 = puVar7;
  do {
    *puVar2 = 0;
    *(undefined *)(puVar2 + 1) = 0;
    puVar2 = (undefined8 *)((longlong)puVar2 + 10);
    lVar4 = lVar4 + -1;
  } while (lVar4 != 0);
  puVar5 = (undefined8 *)((longlong)param_1 + 0xc1);
  lVar4 = 4;
  puVar2 = puVar5;
  do {
    *puVar2 = 0;
    *(undefined *)(puVar2 + 1) = 0;
    puVar2 = (undefined8 *)((longlong)puVar2 + 10);
    lVar4 = lVar4 + -1;
  } while (lVar4 != 0);
  *(undefined *)((longlong)param_1 + 0xe9) = 0;
  *(undefined2 *)((longlong)param_1 + 0xeb) = 0;
  do {
    func_0x0001471b3c70(lVar1);
    func_0x0001471b40a0(puVar7);
    lVar1 = lVar1 + 0xc;
    puVar7 = (undefined8 *)((longlong)puVar7 + 10);
    lVar8 = lVar8 + -1;
  } while (lVar8 != 0);
  lVar1 = 4;
  do {
    func_0x0001471b40a0(puVar5);
    puVar5 = (undefined8 *)((longlong)puVar5 + 10);
    lVar1 = lVar1 + -1;
  } while (lVar1 != 0);
  *(undefined *)((longlong)param_1 + 0xe9) = 0;
  *(undefined2 *)((longlong)param_1 + 0xeb) = 0;
  param_1[0x1e] = 0;
  param_1[0x1f] = 0;
  param_1[0x20] = 0;
  param_1[0x21] = 0;
  param_1[0x22] = 0;
  lVar1 = func_0x000146e8ba20(0x30);
  *(longlong *)lVar1 = lVar1;
  *(longlong *)(lVar1 + 8) = lVar1;
  *(longlong *)(lVar1 + 0x10) = lVar1;
  *(undefined2 *)(lVar1 + 0x18) = 0x101;
  param_1[0x21] = lVar1;
  *(undefined2 *)(param_1 + 0x23) = 0xffff;
  param_1[0x24] = 0;
  param_1[0x25] = 0;
  func_0x000146e9f7d0(param_1 + 0x29);
  *(undefined *)((longlong)param_1 + 0x16a) = 0;
  lVar1 = func_0x000146e8ba20(0x1b0);
  if (lVar1 != 0) {
    uVar3 = FUN_141c0d850(lVar1,0x67);
  }
  param_1[0xb] = uVar3;
  *(undefined4 *)(param_1 + 0x26) = 4;
  *(undefined8 *)((longlong)param_1 + 0x134) = 4;
  *(undefined *)((longlong)param_1 + 0x13c) = 0;
  func_0x000146e9fbc0(param_1 + 0x29);
  *(undefined2 *)(param_1 + 0x2d) = 1;
  *(undefined4 *)(param_1 + 0x28) = 0;
  return param_1;
}


