// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141c3cf60 FUN_141c3cf60

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c3cf60(void)

{
  char cVar1;
  bool bVar2;
  code *pcVar3;
  undefined8 *puVar4;
  undefined8 *puVar5;
  undefined8 uVar6;
  undefined8 *puVar7;
  undefined8 *puVar8;
  undefined8 in_R9;
  undefined8 *puStack_38;
  undefined8 uStack_30;
  undefined8 *puStack_28;
  uint uStack_20;
  undefined4 uStack_1c;
  
  cVar1 = *(char *)((longlong)(undefined8 *)_DAT_14e652c80[1] + 0x19);
  puVar5 = (undefined8 *)_DAT_14e652c80[1];
  puVar8 = _DAT_14e652c80;
  while (puVar7 = puVar5, cVar1 == '\0') {
    if (*(int *)((longlong)puVar7 + 0x1c) < 0x8f5) {
      puVar5 = (undefined8 *)puVar7[2];
      puVar7 = puVar8;
    }
    else {
      puVar5 = (undefined8 *)*puVar7;
    }
    cVar1 = *(char *)((longlong)puVar5 + 0x19);
    puVar8 = puVar7;
  }
  if (((*(char *)((longlong)puVar8 + 0x19) == '\0') && (*(int *)((longlong)puVar8 + 0x1c) < 0x8f6))
     && (puVar8 != _DAT_14e652c80)) {
    return;
  }
  uStack_30 = 0x141c3cfb4;
  uVar6 = func_0x000146d74000();
  uStack_30 = 0x141c3cfc1;
  func_0x000146d746e0(uVar6,0x8f5);
  uStack_30 = 0x141c3cfc6;
  func_0x000146d75af0();
  puVar7 = _DAT_14e652c80;
  uVar6 = 0xfffffffffffffffe;
  puVar5 = (undefined8 *)_DAT_14e652c80[1];
  uStack_20 = 0;
  cVar1 = *(char *)((longlong)puVar5 + 0x19);
  puStack_28 = puVar5;
  puVar8 = _DAT_14e652c80;
  while (puVar4 = puVar5, cVar1 == '\0') {
    bVar2 = 0x8f4 < *(int *)((longlong)puVar4 + 0x1c);
    if (bVar2) {
      puVar5 = (undefined8 *)*puVar4;
      puVar8 = puVar4;
    }
    else {
      puVar5 = (undefined8 *)puVar4[2];
    }
    uStack_20 = (uint)bVar2;
    cVar1 = *(char *)((longlong)puVar5 + 0x19);
    puStack_28 = puVar4;
  }
  if ((*(char *)((longlong)puVar8 + 0x19) != '\0') || (0x8f5 < *(int *)((longlong)puVar8 + 0x1c))) {
    if (_DAT_14e652c88 == 0x7ffffffffffffff) {
      func_0x00014014f360();
      pcVar3 = (code *)swi(3);
      (*pcVar3)();
      return;
    }
    puStack_38 = (undefined8 *)&DAT_14e652c80;
    uStack_30 = 0;
    puVar5 = (undefined8 *)func_0x000146e8ba20(0x20);
    *(undefined4 *)((longlong)puVar5 + 0x1c) = 0x8f5;
    *puVar5 = puVar7;
    puVar5[1] = puVar7;
    puVar5[2] = puVar7;
    *(undefined2 *)(puVar5 + 3) = 0;
    puStack_38 = puStack_28;
    uStack_30 = CONCAT44(uStack_1c,uStack_20);
    func_0x00014014f0e0(&DAT_14e652c80,&puStack_38,puVar5,in_R9,uVar6);
  }
  return;
}



// FUNCTION 140696780 FUN_140696780

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_140696780(longlong *param_1,undefined8 param_2)

{
  code *pcVar1;
  longlong lVar2;
  longlong *plVar3;
  char cVar4;
  undefined uVar5;
  undefined4 uVar6;
  int iVar7;
  undefined8 uVar8;
  undefined8 *puVar9;
  undefined8 ****ppppuVar10;
  longlong lVar11;
  undefined8 ****ppppuVar12;
  ulonglong uVar13;
  ulonglong uVar14;
  undefined2 *puVar15;
  undefined *puVar16;
  int iVar17;
  uint uVar18;
  ulonglong uVar19;
  ulonglong uVar20;
  undefined auStack_168 [32];
  undefined auStack_148 [4];
  undefined4 uStack_144;
  longlong *plStack_140;
  longlong lStack_138;
  undefined8 ****appppuStack_130 [2];
  longlong lStack_120;
  undefined8 uStack_118;
  undefined8 ***apppuStack_110 [2];
  undefined8 uStack_100;
  undefined8 uStack_f8;
  undefined8 uStack_f0;
  ulonglong auStack_e8 [2];
  undefined8 uStack_d8;
  ulonglong uStack_d0;
  undefined8 ****ppppuStack_c8;
  undefined4 uStack_c0;
  undefined4 uStack_bc;
  ulonglong uStack_b8;
  ulonglong uStack_b0;
  undefined8 ****appppuStack_a8 [2];
  longlong lStack_98;
  ulonglong uStack_90;
  undefined2 uStack_88;
  undefined6 uStack_86;
  undefined8 uStack_78;
  ulonglong uStack_70;
  undefined2 uStack_68;
  undefined6 uStack_66;
  undefined8 uStack_58;
  ulonglong uStack_50;
  ulonglong uStack_48;
  
  uStack_f0 = 0xfffffffffffffffe;
  uStack_48 = _DAT_14dca9408 ^ (ulonglong)auStack_168;
  uStack_144 = 0;
  plStack_140 = param_1;
  uStack_118 = param_2;
  if (((param_1[0x10] == 0) || (cVar4 = (**(code **)(*param_1 + 0x50))(), cVar4 == '\0')) ||
     (lStack_138 = func_0x00014667eb40(_DAT_14e683c78,0x282), lStack_138 == 0)) goto LAB_140696eea;
  lStack_120 = func_0x000142501590(lStack_138);
  uVar8 = FUN_140696290();
  uVar6 = func_0x0001406996d0(uVar8);
  uVar8 = FUN_140696290();
  func_0x000140699030(uVar8,&uStack_68,uVar6);
  lVar11 = param_1[0x10];
  appppuStack_130[0] = apppuStack_110;
  uVar8 = func_0x000146e8c7d0(&UNK_1492e14d8);
  apppuStack_110[0] = (undefined8 ***)0x0;
  uStack_100 = 0;
  uStack_f8 = 7;
  func_0x00014014c8b0(apppuStack_110,uVar8);
  puVar9 = (undefined8 *)func_0x0001476e0090(lVar11,apppuStack_110);
  ppppuStack_c8 = (undefined8 ****)0x0;
  uStack_b8 = 0;
  uStack_b0 = 0;
  uVar13 = puVar9[2];
  if (7 < (ulonglong)puVar9[3]) {
    puVar9 = (undefined8 *)*puVar9;
  }
  if (uVar13 < 8) {
    ppppuVar10 = (undefined8 ****)*puVar9;
    uStack_c0 = *(undefined4 *)(puVar9 + 1);
    uStack_bc = *(undefined4 *)((longlong)puVar9 + 0xc);
    uStack_b0 = 7;
    ppppuStack_c8 = ppppuVar10;
  }
  else {
    uVar20 = uVar13 | 7;
    if (0x7ffffffffffffffe < uVar20) {
      uVar20 = 0x7ffffffffffffffe;
    }
    ppppuVar10 = (undefined8 ****)func_0x00014014cb50(&ppppuStack_c8,uVar20 + 1);
    ppppuStack_c8 = ppppuVar10;
    func_0x000148aa1e60(ppppuVar10,puVar9,uVar13 * 2 + 2);
    uStack_b0 = uVar20;
  }
  uVar20 = uStack_b0;
  uStack_b8 = uVar13;
  lVar11 = func_0x000146e8c7d0(&UNK_1491ce988);
  uVar19 = 0xffffffffffffffff;
  do {
    uVar19 = uVar19 + 1;
  } while (*(short *)(lVar11 + uVar19 * 2) != 0);
  if (0x7ffffffffffffffe - uVar13 < uVar19) {
LAB_140696f17:
    func_0x00014014ac70();
LAB_140696f1d:
    func_0x000148aaf304();
    pcVar1 = (code *)swi(3);
    (*pcVar1)();
    return;
  }
  appppuStack_130[0] = &ppppuStack_c8;
  if (7 < uVar20) {
    appppuStack_130[0] = ppppuVar10;
  }
  appppuStack_a8[0] = (undefined8 ****)0x0;
  lStack_98 = 0;
  uStack_90 = 0;
  uVar14 = 7;
  ppppuVar12 = appppuStack_a8;
  if (7 < uVar13 + uVar19) {
    uVar14 = uVar13 + uVar19 | 7;
    if (uVar14 < 0x7fffffffffffffff) {
      if (uVar14 < 10) {
        uVar14 = 10;
      }
    }
    else {
      uVar14 = 0x7ffffffffffffffe;
    }
    ppppuVar12 = (undefined8 ****)func_0x00014014cb50(appppuStack_a8,uVar14 + 1);
    appppuStack_a8[0] = ppppuVar12;
  }
  lStack_98 = uVar13 + uVar19;
  uStack_90 = uVar14;
  func_0x000148aa1e60(ppppuVar12,appppuStack_130[0],uVar13 * 2);
  func_0x000148aa1e60(uVar13 * 2 + (longlong)ppppuVar12,lVar11,uVar19 * 2);
  *(undefined2 *)((longlong)ppppuVar12 + (uVar13 + uVar19) * 2) = 0;
  iVar17 = 1;
  uStack_144 = 1;
  func_0x000140695bf0(&uStack_88,appppuStack_a8,&uStack_68);
  uStack_144 = 0;
  if (7 < uStack_90) {
    uVar13 = uStack_90 * 2 + 2;
    ppppuVar12 = appppuStack_a8[0];
    if (0xfff < uVar13) {
      uVar13 = uStack_90 * 2 + 0x29;
      ppppuVar12 = (undefined8 ****)appppuStack_a8[0][-1];
      if (0x1f < (ulonglong)((longlong)appppuStack_a8[0] + (-8 - (longlong)ppppuVar12)))
      goto LAB_140696f1d;
    }
    func_0x000146e9f3a0(ppppuVar12,uVar13);
  }
  uVar13 = 0;
  lStack_98 = 0;
  uStack_90 = 7;
  appppuStack_a8[0] = (undefined8 ****)((ulonglong)appppuStack_a8[0] & 0xffffffffffff0000);
  uVar8 = FUN_140696290();
  uVar5 = func_0x000140698540(uVar8);
  puVar15 = &uStack_88;
  if (7 < uStack_70) {
    puVar15 = (undefined2 *)CONCAT62(uStack_86,uStack_88);
  }
  uVar8 = func_0x00014723c170(0x605b942);
  uVar8 = func_0x000146e8cf20(appppuStack_130,uVar8,puVar15);
  uVar8 = func_0x00014014f430(uVar8);
  lVar11 = lStack_120;
  func_0x00014014c8b0(lStack_120,uVar8);
  func_0x000146e8c910(appppuStack_130);
  func_0x00014014c8b0(lVar11 + 0x20,&DAT_14baf7f08);
  plVar3 = plStack_140;
  *(undefined *)(lVar11 + 0x60) = 1;
  auStack_148[0] = uVar5;
  uVar6 = (**(code **)(*(longlong *)plStack_140[0x10] + 0x80))
                    ((longlong *)plStack_140[0x10],auStack_148);
  *(undefined4 *)(lVar11 + 0x70) = uVar6;
  auStack_148[0] = uVar5;
  uVar6 = (**(code **)(*(longlong *)plVar3[0x10] + 0x88))((longlong *)plVar3[0x10],auStack_148);
  *(undefined4 *)(lVar11 + 0x68) = uVar6;
  auStack_148[0] = uVar5;
  uVar6 = (**(code **)(*(longlong *)plVar3[0x10] + 0x90))((longlong *)plVar3[0x10],auStack_148);
  *(undefined4 *)(lVar11 + 0x6c) = uVar6;
  uVar8 = FUN_140696290();
  cVar4 = func_0x00014069a040(uVar8);
  if (cVar4 == '\0') {
    uVar8 = FUN_140696290();
    cVar4 = func_0x00014069a0a0(uVar8);
    if (cVar4 != '\0') goto LAB_140696bc5;
    puVar16 = (undefined *)(lVar11 + 0xd8);
    uVar19 = uVar13;
    do {
      *puVar16 = 0;
      *(undefined4 *)(puVar16 + 4) = 0;
      puVar16[8] = 0;
      if ((uVar19 & 0xfffffffb) != 0) {
        *puVar16 = 1;
        if (_DAT_14e638998 == 0) {
          plStack_140 = (longlong *)func_0x000146e8ba20(0x158);
          uVar14 = uVar13;
          if (plStack_140 != (longlong *)0x0) {
            uVar14 = func_0x000140697290(plStack_140);
          }
          _DAT_14e638998 = uVar14;
          (***(code ***)(uVar14 + 0x50))();
        }
        uVar6 = func_0x000140698510(_DAT_14e638998,uVar19);
        *(undefined4 *)(puVar16 + 4) = uVar6;
      }
      uVar18 = (int)uVar19 + 1;
      uVar19 = (ulonglong)uVar18;
      puVar16 = puVar16 + 0xc;
    } while ((int)uVar18 < 5);
  }
  else {
LAB_140696bc5:
    puVar16 = (undefined *)(lVar11 + 0xe0);
    lVar11 = 5;
    do {
      *(undefined4 *)(puVar16 + -4) = 0;
      *puVar16 = 0;
      puVar16[-8] = 1;
      if (_DAT_14e638998 == 0) {
        plStack_140 = (longlong *)func_0x000146e8ba20(0x158);
        uVar19 = uVar13;
        if (plStack_140 != (longlong *)0x0) {
          uVar19 = func_0x000140697290(plStack_140);
        }
        _DAT_14e638998 = uVar19;
        (***(code ***)(uVar19 + 0x50))();
      }
      uVar6 = func_0x000140698510(_DAT_14e638998,iVar17);
      *(undefined4 *)(puVar16 + -4) = uVar6;
      if (_DAT_14e638998 == 0) {
        lStack_120 = func_0x000146e8ba20(0x158);
        uVar19 = uVar13;
        if (lStack_120 != 0) {
          uVar19 = func_0x000140697290(lStack_120);
        }
        _DAT_14e638998 = uVar19;
        (***(code ***)(uVar19 + 0x50))();
      }
      cVar4 = func_0x00014069a030(_DAT_14e638998,iVar17);
      *puVar16 = cVar4 == '\0';
      if (_DAT_14e638998 == 0) {
        appppuStack_130[0] = (undefined8 ****)func_0x000146e8ba20(0x158);
        uVar19 = uVar13;
        if (appppuStack_130[0] != (undefined8 ****)0x0) {
          uVar19 = func_0x000140697290(appppuStack_130[0]);
        }
        _DAT_14e638998 = uVar19;
        (***(code ***)(uVar19 + 0x50))();
      }
      iVar7 = func_0x000140698520(_DAT_14e638998);
      if (iVar7 == iVar17) {
        *puVar16 = 0;
      }
      iVar17 = iVar17 + 1;
      puVar16 = puVar16 + 0xc;
      lVar11 = lVar11 + -1;
    } while (lVar11 != 0);
  }
  if (_DAT_14e638998 == 0) {
    plStack_140 = (longlong *)func_0x000146e8ba20(0x158);
    if (plStack_140 != (longlong *)0x0) {
      uVar13 = func_0x000140697290(plStack_140);
    }
    _DAT_14e638998 = uVar13;
    (***(code ***)(uVar13 + 0x50))();
  }
  uVar6 = func_0x000140698520(_DAT_14e638998);
  uStack_d8 = 0;
  uStack_d0 = 7;
  auStack_e8[0] = 0;
  func_0x00014014c8b0(auStack_e8,&DAT_14baf7f08);
  cVar4 = func_0x00014250e050(plVar3,uVar6);
  if (cVar4 == '\0') {
    cVar4 = func_0x00014250e030(plVar3,uVar6);
    if (cVar4 != '\0') {
      uVar8 = 0x605b3ec;
      goto LAB_140696d7d;
    }
  }
  else {
    uVar8 = 0x605b3eb;
LAB_140696d7d:
    uVar8 = func_0x00014723c170(uVar8);
    func_0x00014014c8b0(auStack_e8,uVar8);
    func_0x000142505c40(lStack_138,auStack_e8);
  }
  func_0x00014250f330(plVar3,uStack_118);
  if (7 < uStack_d0) {
    if (uStack_d0 * 2 + 2 < 0x1000) {
LAB_140696de5:
      func_0x000146e9f3a0();
      goto LAB_140696dea;
    }
    if ((auStack_e8[0] - *(longlong *)(auStack_e8[0] - 8)) - 8 < 0x20) goto LAB_140696de5;
    func_0x000148aaf304(*(longlong *)(auStack_e8[0] - 8),uStack_d0 * 2 + 0x29);
LAB_140696f29:
    func_0x000148aaf304();
LAB_140696f2f:
    func_0x000148aaf304();
    pcVar1 = (code *)swi(3);
    (*pcVar1)();
    return;
  }
LAB_140696dea:
  uStack_d8 = 0;
  uStack_d0 = 7;
  auStack_e8[0] = auStack_e8[0] & 0xffffffffffff0000;
  if (7 < uStack_70) {
    uVar13 = uStack_70 * 2 + 2;
    lVar2 = CONCAT62(uStack_86,uStack_88);
    lVar11 = lVar2;
    if (0xfff < uVar13) {
      uVar13 = uStack_70 * 2 + 0x29;
      lVar11 = *(longlong *)(lVar2 + -8);
      if (0x1f < (lVar2 - lVar11) - 8U) goto LAB_140696f29;
    }
    func_0x000146e9f3a0(lVar11,uVar13);
  }
  uStack_78 = 0;
  uStack_70 = 7;
  uStack_88 = 0;
  if (7 < uVar20) {
    uVar13 = uVar20 * 2 + 2;
    ppppuVar12 = ppppuVar10;
    if (0xfff < uVar13) {
      uVar13 = uVar20 * 2 + 0x29;
      ppppuVar12 = (undefined8 ****)ppppuVar10[-1];
      if (0x1f < (ulonglong)((longlong)ppppuVar10 + (-8 - (longlong)ppppuVar12)))
      goto LAB_140696f2f;
    }
    func_0x000146e9f3a0(ppppuVar12,uVar13);
  }
  uStack_b8 = 0;
  uStack_b0 = 7;
  ppppuStack_c8 = (undefined8 ****)((ulonglong)ppppuStack_c8 & 0xffffffffffff0000);
  if (7 < uStack_50) {
    if (0xfff < uStack_50 * 2 + 2) {
      lVar11 = *(longlong *)(CONCAT62(uStack_66,uStack_68) + -8);
      if (0x1f < (CONCAT62(uStack_66,uStack_68) - lVar11) - 8U) {
        func_0x000148aaf304(lVar11,uStack_50 * 2 + 0x29);
        goto LAB_140696f17;
      }
    }
    func_0x000146e9f3a0();
  }
  uStack_58 = 0;
  uStack_50 = 7;
  uStack_68 = 0;
LAB_140696eea:
  func_0x000148860610(uStack_48 ^ (ulonglong)auStack_168);
  return;
}



// FUNCTION 140696290 FUN_140696290

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

longlong FUN_140696290(void)

{
  longlong lVar1;
  
  if (_DAT_14e638998 == 0) {
    lVar1 = func_0x000146e8ba20(0x158);
    if (lVar1 != 0) {
      lVar1 = func_0x000140697290(lVar1);
    }
    _DAT_14e638998 = lVar1;
    (***(code ***)(lVar1 + 0x50))();
  }
  return _DAT_14e638998;
}



// FUNCTION 1424fd320 FUN_1424fd320

void FUN_1424fd320(uint param_1,char param_2)

{
  undefined8 uVar1;
  uint auStackX_10 [6];
  undefined4 uStack_18;
  undefined2 uStack_14;
  undefined4 uStack_12;
  undefined uStack_e;
  undefined2 uStack_c;
  
  auStackX_10[0] = param_1;
  func_0x0001403a7590(&DAT_14e652c80,auStackX_10);
  if (param_2 != '\0') {
    auStackX_10[0] = auStackX_10[0] & 0xffff0000;
    func_0x000146ea1920(auStackX_10);
    if ((short)auStackX_10[0] == 0x6b) {
      uStack_18 = 0;
      uStack_14 = 0xff;
      uStack_12 = 0;
      uStack_e = 0;
      uStack_c = 0;
      func_0x000146ea0be0(&uStack_18,0xe);
      uVar1 = FUN_140696290();
      func_0x00014069a360(uVar1,&uStack_18);
    }
  }
  return;
}



// FUNCTION 141d36bb0 FUN_141d36bb0

/* WARNING: Possible PIC construction at 0x000141d36c47: Changing call to branch */
/* WARNING: Removing unreachable block (ram,0x000141d36c4c) */

void FUN_141d36bb0(longlong param_1)

{
  longlong lVar1;
  
  *(undefined *)(param_1 + 0xcf) = 0;
  *(undefined4 *)(param_1 + 0x6b) = 0xe;
  *(undefined4 *)(param_1 + 0x6f) = 4;
  *(undefined2 *)(param_1 + 0x69) = 0xffff;
  *(undefined8 *)(param_1 + 0x73) = 0xffffffffffffffff;
  for (lVar1 = param_1 + 0x7b; lVar1 != param_1 + 0x9f; lVar1 = lVar1 + 0xc) {
    func_0x0001471b3c70(lVar1);
  }
  if (param_1 + 0x9fU <= param_1 + 0xcfU) {
    func_0x000148aa2510(param_1 + 0x9fU,0,0x30);
  }
  *(undefined *)(param_1 + 0xf1) = 1;
  func_0x000146e9fcf0(param_1 + 0xf8,0);
  *(uint *)(param_1 + 0xfc) = *(uint *)(param_1 + 0xfc) & 0xfffffff9;
  return;
}


