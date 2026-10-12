// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141d362a0 FUN_141d362a0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141d362a0(longlong param_1,undefined4 *param_2)

{
  undefined4 uVar1;
  bool bVar2;
  bool bVar3;
  char cVar4;
  int iVar5;
  undefined4 uVar6;
  undefined4 uVar7;
  undefined8 uVar8;
  int iVar9;
  
  iVar5 = 1;
  func_0x000142ac27e0(_DAT_14e683c40,1);
  if ((*(int *)(param_1 + 0x73) == -1) && (*(int *)((longlong)param_2 + 10) == 0)) {
    bVar3 = true;
    iVar9 = 0;
LAB_141d362e6:
    bVar2 = false;
  }
  else {
    iVar9 = *(int *)((longlong)param_2 + 10);
    bVar3 = false;
    if (iVar9 <= *(int *)(param_1 + 0x73)) goto LAB_141d362e6;
    bVar2 = true;
  }
  if ((*(int *)((longlong)param_2 + 2) == 0xe) && (iVar9 == -1)) {
    if (*(int *)((longlong)param_2 + 6) != 4) {
LAB_141d362fc:
      if (*(int *)((longlong)param_2 + 6) != 3) goto LAB_141d36334;
      iVar5 = 3;
      goto LAB_141d36343;
    }
  }
  else {
    if (iVar9 != 2) goto LAB_141d362fc;
    if (*(int *)((longlong)param_2 + 6) == 3) {
      iVar5 = 2;
      goto LAB_141d36350;
    }
    if (*(int *)((longlong)param_2 + 6) == 1) {
      iVar5 = 2;
      goto LAB_141d36350;
    }
LAB_141d36334:
    iVar5 = 0;
    if (*(int *)((longlong)param_2 + 2) == 5) {
      iVar5 = 2;
    }
LAB_141d36343:
    if (iVar9 != -1) goto LAB_141d36350;
  }
  func_0x000141d36bb0(param_1);
LAB_141d36350:
  uVar6 = param_2[1];
  uVar7 = param_2[2];
  uVar1 = param_2[3];
  *(undefined4 *)(param_1 + 0x69) = *param_2;
  *(undefined4 *)(param_1 + 0x6d) = uVar6;
  *(undefined4 *)(param_1 + 0x71) = uVar7;
  *(undefined4 *)(param_1 + 0x75) = uVar1;
  uVar6 = param_2[5];
  uVar7 = param_2[6];
  uVar1 = param_2[7];
  *(undefined4 *)(param_1 + 0x79) = param_2[4];
  *(undefined4 *)(param_1 + 0x7d) = uVar6;
  *(undefined4 *)(param_1 + 0x81) = uVar7;
  *(undefined4 *)(param_1 + 0x85) = uVar1;
  uVar6 = param_2[9];
  uVar7 = param_2[10];
  uVar1 = param_2[0xb];
  *(undefined4 *)(param_1 + 0x89) = param_2[8];
  *(undefined4 *)(param_1 + 0x8d) = uVar6;
  *(undefined4 *)(param_1 + 0x91) = uVar7;
  *(undefined4 *)(param_1 + 0x95) = uVar1;
  *(undefined4 *)(param_1 + 0x99) = param_2[0xc];
  *(undefined2 *)(param_1 + 0x9d) = *(undefined2 *)(param_2 + 0xd);
  if ((*(char *)(param_1 + 0xcf) == '\0') &&
     (((*(undefined *)(param_1 + 0xcf) = 1, *(int *)(param_1 + 0x6b) != 0xe ||
       (*(int *)(param_1 + 0x73) != -1)) || (*(int *)(param_1 + 0x6f) != 4)))) {
    if (*(longlong **)(param_1 + 0x60) != (longlong *)0x0) {
      (**(code **)(**(longlong **)(param_1 + 0x60) + 0x20))();
      (**(code **)(**(longlong **)(param_1 + 0x60) + 0x30))
                (*(longlong **)(param_1 + 0x60),*(int *)(param_1 + 0x6f));
      (**(code **)(**(longlong **)(param_1 + 0x60) + 0x40))
                (*(longlong **)(param_1 + 0x60),*(undefined4 *)(param_1 + 0x73));
      if (((*(longlong **)(param_1 + 0x60) != (longlong *)0x0) &&
          (cVar4 = (**(code **)(**(longlong **)(param_1 + 0x60) + 0x198))(), cVar4 != '\0')) &&
         ((*(int *)(param_1 + 0x6f) != 3 || (*(int *)(param_1 + 0x73) != 2)))) {
        func_0x000141d355c0(*(undefined8 *)(param_1 + 0x60));
      }
    }
  }
  *(undefined *)(param_1 + 0xf1) = 1;
  if ((iVar5 == 1) || (iVar5 == 2)) {
    cVar4 = func_0x000146682140(_DAT_14e683c78,0x2bf,1);
    if (cVar4 != '\0') {
      func_0x000146694510(_DAT_14e683c78,0x2bf,0xffffffff,0,1);
    }
  }
  else {
    if (iVar5 == 3) {
      if (((*(int *)(param_1 + 0x6b) == 0xe) && (*(int *)(param_1 + 0x73) == -1)) &&
         (*(int *)(param_1 + 0x6f) == 4)) {
        cVar4 = func_0x000146682140(_DAT_14e683c78,0x2bf,1);
        if (cVar4 != '\0') {
          func_0x000146694510(_DAT_14e683c78,0x2bf,0xffffffff,0,1);
        }
      }
      else {
        cVar4 = func_0x000146682140(_DAT_14e683c78,0x2bf,1);
        if (cVar4 == '\0') {
          func_0x00014668c520(_DAT_14e683c78,0x2bf,0,0);
        }
      }
    }
    cVar4 = func_0x000145f15310(_DAT_14e683c20);
    if (cVar4 != '\0') {
      iVar5 = func_0x0001459a90f0(_DAT_14e66c090);
      if (iVar5 == 1) {
        if ((bVar3) && (*(char *)(param_1 + 0x68) != '\0')) {
          *(undefined *)(param_1 + 0x68) = 0;
          uVar8 = func_0x000141308bd0(_DAT_14e66c090);
          uVar6 = func_0x000141c4a150(uVar8);
          uVar8 = func_0x000141308bd0(_DAT_14e66c090);
          uVar7 = func_0x000146d01ab0(uVar8);
          cVar4 = func_0x000142abaf50(_DAT_14e683c40,uVar6,uVar7);
          if (cVar4 == '\0') {
            if (_DAT_14e683c78 != 0) {
              uVar8 = func_0x00014723c170(0x605abc6);
              func_0x00014668c520(_DAT_14e683c78,0xb3b,uVar8,0);
            }
          }
          else {
            func_0x000141d42730(*(undefined4 *)(param_1 + 0x73));
          }
        }
      }
      else if ((iVar5 == 3) && (bVar2)) {
        func_0x000146e9fcf0(param_1 + 0xf8,1);
        func_0x000146e9fbd0(param_1 + 0xf8,15000,0);
      }
    }
  }
  return;
}



// FUNCTION 141d35a10 FUN_141d35a10

undefined8 *
FUN_141d35a10(undefined8 *param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  undefined8 uVar1;
  longlong lVar2;
  longlong lVar3;
  longlong lVar4;
  undefined8 uVar5;
  
  uVar5 = 0xfffffffffffffffe;
  func_0x000141d35d60();
  *param_1 = &UNK_14982b580;
  param_1[10] = &UNK_14982b7e8;
  uVar1 = 0;
  param_1[0xb] = 0;
  param_1[0xc] = 0;
  *(undefined *)(param_1 + 0xd) = 0;
  lVar2 = (longlong)param_1 + 0x7b;
  lVar3 = 3;
  lVar4 = lVar2;
  do {
    func_0x0001471b0f60(lVar4);
    lVar4 = lVar4 + 0xc;
    lVar3 = lVar3 + -1;
  } while (lVar3 != 0);
  *(undefined2 *)((longlong)param_1 + 0x69) = 0xffff;
  *(undefined4 *)((longlong)param_1 + 0x6b) = 0xe;
  *(undefined4 *)((longlong)param_1 + 0x6f) = 4;
  *(undefined8 *)((longlong)param_1 + 0x73) = 0xffffffffffffffff;
  for (; lVar2 != (longlong)param_1 + 0x9f; lVar2 = lVar2 + 0xc) {
    func_0x0001471b3c70(lVar2);
  }
  if ((longlong)param_1 + 0x9fU <= (longlong)param_1 + 0xcfU) {
    func_0x000148aa2510((longlong)param_1 + 0x9fU,0,0x30,param_4,uVar5);
  }
  *(undefined *)((longlong)param_1 + 0xcf) = 0;
  func_0x000146e9f7d0(param_1 + 0x1a);
  *(undefined2 *)(param_1 + 0x1e) = 0;
  func_0x000146e9f7d0(param_1 + 0x1f);
  lVar2 = func_0x000146e8ba20(0x1b0);
  if (lVar2 != 0) {
    uVar1 = func_0x000141d31e40(lVar2,0x69);
  }
  param_1[0xc] = uVar1;
  func_0x000146e9fbc0(param_1 + 0x1a);
  return param_1;
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


