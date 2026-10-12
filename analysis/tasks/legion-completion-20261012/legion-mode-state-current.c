// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 142ac2290 FUN_142ac2290

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_142ac2290(longlong param_1,int param_2)

{
  undefined8 *puVar1;
  longlong *plVar2;
  code *pcVar3;
  undefined8 *puVar4;
  int *piVar5;
  int *piVar6;
  char cVar7;
  int iVar8;
  undefined4 uVar9;
  int iVar10;
  undefined8 uVar11;
  longlong lVar12;
  undefined8 *puVar13;
  undefined8 *puVar14;
  ulonglong uVar15;
  int *piVar16;
  undefined4 extraout_XMM0_Da;
  undefined4 auStackX_18 [2];
  undefined auStack_20 [16];
  longlong lStack_10;
  
  iVar8 = func_0x000142ab29c0();
  switch(iVar8) {
  case 0:
    uVar11 = func_0x000142ab48f0();
    func_0x00014331e2c0(uVar11,param_2);
    break;
  case 1:
    uVar11 = func_0x000142ab49b0();
    func_0x000144533fa0(uVar11,param_2);
    break;
  case 2:
    uVar11 = func_0x000142ab4950();
    func_0x000142ca2e10(uVar11,param_2);
    break;
  case 3:
    uVar11 = func_0x000142a25ca0();
    func_0x000142a2ca20(uVar11,param_2);
    break;
  case 7:
    uVar11 = func_0x0001428f61e0();
    func_0x0001428f1270(uVar11,param_2);
    return;
  case 10:
    uVar11 = func_0x0001425b1560();
    func_0x00014276dcd0(uVar11,param_2);
    break;
  case 0x10:
    uVar11 = func_0x00014205e170();
    func_0x00014206e7c0(uVar11,param_2);
    break;
  case 0x11:
    uVar11 = func_0x000141a3a180();
    func_0x00014212ed60(uVar11,param_2);
    break;
  case 0x14:
    uVar11 = func_0x000140efb2c0();
    func_0x000146dc5cc0(uVar11,param_2);
    break;
  case 0x15:
    uVar11 = func_0x000141608600();
    func_0x0001421b9a20(uVar11,param_2);
    break;
  case 0x1b:
    lVar12 = func_0x000140efb260();
    goto code_r0x000142ac239e;
  case 0x1c:
    lVar12 = func_0x0001416085a0();
code_r0x000142ac239e:
    (**(code **)(*(longlong *)(lVar12 + 8) + 0x38))((longlong *)(lVar12 + 8),param_2);
  }
  puVar1 = *(undefined8 **)(param_1 + 0x230);
  cVar7 = *(char *)((longlong)(undefined8 *)puVar1[1] + 0x19);
  puVar4 = puVar1;
  puVar14 = (undefined8 *)puVar1[1];
  while (cVar7 == '\0') {
    if (*(int *)(puVar14 + 4) < iVar8) {
      puVar13 = (undefined8 *)puVar14[2];
      puVar14 = puVar4;
    }
    else {
      puVar13 = (undefined8 *)*puVar14;
    }
    puVar4 = puVar14;
    puVar14 = puVar13;
    cVar7 = *(char *)((longlong)puVar13 + 0x19);
  }
  if ((((*(char *)((longlong)puVar4 + 0x19) == '\0') && (*(int *)(puVar4 + 4) <= iVar8)) &&
      (puVar4 != puVar1)) && (plVar2 = (longlong *)puVar4[5], plVar2 != (longlong *)0x0)) {
    (**(code **)(*plVar2 + 0x38))(plVar2,param_2);
  }
  uVar9 = func_0x000145241bf0(_DAT_14e66c090);
  auStackX_18[0] = uVar9;
  iVar10 = func_0x000142aacc00(param_1 + 0x148,auStackX_18);
  if ((iVar10 < 2) || (param_2 != 0)) {
    puVar1 = *(undefined8 **)(param_1 + 0x138);
    cVar7 = *(char *)((longlong)(undefined8 *)puVar1[1] + 0x19);
    puVar4 = puVar1;
    puVar14 = (undefined8 *)puVar1[1];
    while (cVar7 == '\0') {
      if (*(int *)(puVar14 + 4) < iVar8) {
        puVar13 = (undefined8 *)puVar14[2];
        puVar14 = puVar4;
      }
      else {
        puVar13 = (undefined8 *)*puVar14;
      }
      puVar4 = puVar14;
      puVar14 = puVar13;
      cVar7 = *(char *)((longlong)puVar13 + 0x19);
    }
    if (((*(char *)((longlong)puVar4 + 0x19) == '\0') && (*(int *)(puVar4 + 4) <= iVar8)) &&
       ((puVar4 != puVar1 && (plVar2 = (longlong *)puVar4[5], plVar2 != (longlong *)0x0)))) {
      (**(code **)(*plVar2 + 0x20))(plVar2,param_2);
    }
  }
  else {
    auStack_20 = ZEXT816(0);
    lStack_10 = 0;
    cVar7 = func_0x000142ab6760(param_1,uVar9,auStack_20);
    piVar5 = (int *)auStack_20._0_8_;
    uVar9 = extraout_XMM0_Da;
    if (cVar7 == '\0') {
      puVar1 = *(undefined8 **)(param_1 + 0x138);
      cVar7 = *(char *)((longlong)(undefined8 *)puVar1[1] + 0x19);
      puVar4 = puVar1;
      puVar14 = (undefined8 *)puVar1[1];
      while (cVar7 == '\0') {
        if (*(int *)(puVar14 + 4) < iVar8) {
          puVar13 = (undefined8 *)puVar14[2];
          puVar14 = puVar4;
        }
        else {
          puVar13 = (undefined8 *)*puVar14;
        }
        puVar4 = puVar14;
        puVar14 = puVar13;
        cVar7 = *(char *)((longlong)puVar13 + 0x19);
      }
      if ((((*(char *)((longlong)puVar4 + 0x19) == '\0') && (*(int *)(puVar4 + 4) <= iVar8)) &&
          (puVar4 != puVar1)) && (plVar2 = (longlong *)puVar4[5], plVar2 != (longlong *)0x0)) {
        uVar9 = (**(code **)(*plVar2 + 0x20))(plVar2,0);
      }
    }
    else {
      piVar6 = (int *)auStack_20._8_8_;
      for (piVar16 = (int *)auStack_20._0_8_; piVar16 != piVar6; piVar16 = piVar16 + 1) {
        puVar1 = *(undefined8 **)(param_1 + 0x138);
        cVar7 = *(char *)((longlong)(undefined8 *)puVar1[1] + 0x19);
        puVar4 = puVar1;
        puVar14 = (undefined8 *)puVar1[1];
        while (cVar7 == '\0') {
          if (*(int *)(puVar14 + 4) < *piVar16) {
            puVar13 = (undefined8 *)puVar14[2];
            puVar14 = puVar4;
          }
          else {
            puVar13 = (undefined8 *)*puVar14;
          }
          puVar4 = puVar14;
          puVar14 = puVar13;
          cVar7 = *(char *)((longlong)puVar13 + 0x19);
        }
        if (((*(char *)((longlong)puVar4 + 0x19) == '\0') && (*(int *)(puVar4 + 4) <= *piVar16)) &&
           ((puVar4 != puVar1 && (plVar2 = (longlong *)puVar4[5], plVar2 != (longlong *)0x0)))) {
          uVar9 = (**(code **)(*plVar2 + 0x20))(plVar2,0);
        }
      }
    }
    if (piVar5 != (int *)0x0) {
      uVar15 = lStack_10 - (longlong)piVar5 & 0xfffffffffffffffc;
      piVar16 = piVar5;
      if (0xfff < uVar15) {
        piVar16 = *(int **)(piVar5 + -2);
        if (0x1f < (ulonglong)((longlong)piVar5 + (-8 - (longlong)piVar16))) {
          func_0x000148aaf304(uVar9,uVar15 + 0x27);
          pcVar3 = (code *)swi(3);
          (*pcVar3)();
          return;
        }
      }
      func_0x000146e9f3a0(piVar16);
    }
  }
  return;
}



// FUNCTION 142ac2220 FUN_142ac2220

void FUN_142ac2220(longlong param_1,undefined4 param_2)

{
  char cVar1;
  undefined8 *puVar2;
  longlong *plVar3;
  undefined8 *puVar4;
  int iVar5;
  undefined8 *puVar6;
  undefined8 *puVar7;
  
  iVar5 = func_0x000142ab29c0();
  puVar2 = *(undefined8 **)(param_1 + 0x138);
  cVar1 = *(char *)((longlong)(undefined8 *)puVar2[1] + 0x19);
  puVar4 = puVar2;
  puVar7 = (undefined8 *)puVar2[1];
  while (cVar1 == '\0') {
    if (*(int *)(puVar7 + 4) < iVar5) {
      puVar6 = (undefined8 *)puVar7[2];
      puVar7 = puVar4;
    }
    else {
      puVar6 = (undefined8 *)*puVar7;
    }
    puVar4 = puVar7;
    puVar7 = puVar6;
    cVar1 = *(char *)((longlong)puVar6 + 0x19);
  }
  if ((((*(char *)((longlong)puVar4 + 0x19) == '\0') && (*(int *)(puVar4 + 4) <= iVar5)) &&
      (puVar4 != puVar2)) && (plVar3 = (longlong *)puVar4[5], plVar3 != (longlong *)0x0)) {
    (**(code **)(*plVar3 + 0x30))(plVar3,param_2);
  }
  return;
}



// FUNCTION 142ac21b0 FUN_142ac21b0

void FUN_142ac21b0(longlong param_1,undefined4 param_2)

{
  char cVar1;
  undefined8 *puVar2;
  longlong *plVar3;
  undefined8 *puVar4;
  int iVar5;
  undefined8 *puVar6;
  undefined8 *puVar7;
  
  iVar5 = func_0x000142ab29c0();
  puVar2 = *(undefined8 **)(param_1 + 0x138);
  cVar1 = *(char *)((longlong)(undefined8 *)puVar2[1] + 0x19);
  puVar4 = puVar2;
  puVar7 = (undefined8 *)puVar2[1];
  while (cVar1 == '\0') {
    if (*(int *)(puVar7 + 4) < iVar5) {
      puVar6 = (undefined8 *)puVar7[2];
      puVar7 = puVar4;
    }
    else {
      puVar6 = (undefined8 *)*puVar7;
    }
    puVar4 = puVar7;
    puVar7 = puVar6;
    cVar1 = *(char *)((longlong)puVar6 + 0x19);
  }
  if ((((*(char *)((longlong)puVar4 + 0x19) == '\0') && (*(int *)(puVar4 + 4) <= iVar5)) &&
      (puVar4 != puVar2)) && (plVar3 = (longlong *)puVar4[5], plVar3 != (longlong *)0x0)) {
    (**(code **)(*plVar3 + 0x40))(plVar3,param_2);
  }
  return;
}



// FUNCTION 14250bc70 FUN_14250bc70

int FUN_14250bc70(longlong param_1)

{
  longlong lVar1;
  undefined8 uVar2;
  
  lVar1 = func_0x000145f0b890();
  if (lVar1 == 0) {
    return *(int *)(param_1 + 0x44);
  }
  uVar2 = func_0x000145f0b890();
  lVar1 = func_0x00014014f460(uVar2);
  if (*(short *)(lVar1 + 0x74) == -1) {
    return 0;
  }
  uVar2 = func_0x000145f0b890();
  lVar1 = func_0x00014014f460(uVar2);
  return (int)*(short *)(lVar1 + 0x74);
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



// FUNCTION 14069b580 FUN_14069b580

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14069b580(void)

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
    uStack_1b = 1;
    uStack_17 = 0xff;
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


