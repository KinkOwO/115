// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141c204b0 FUN_141c204b0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c204b0(undefined8 *param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  longlong *plVar1;
  int *piVar2;
  longlong **pplVar3;
  int iVar4;
  char cVar5;
  longlong *plVar6;
  undefined8 *puVar7;
  longlong **pplVar8;
  longlong lVar9;
  longlong **pplVar10;
  longlong **pplVar11;
  
  *param_1 = &UNK_1497e9fe8;
  param_1[10] = &UNK_1497ea250;
  plVar6 = (longlong *)param_1[0xb];
  if (plVar6 != (longlong *)0x0) {
    (**(code **)(*plVar6 + 8))(plVar6,1,param_3,param_4,0xfffffffffffffffe);
  }
  param_1[0xb] = 0;
  puVar7 = (undefined8 *)param_1[0xc];
  if (puVar7 != (undefined8 *)0x0) {
    (**(code **)*puVar7)(puVar7,1);
  }
  param_1[0xc] = 0;
  lVar9 = _DAT_14e64f750;
  if (_DAT_14e64f750 != 0) {
    func_0x0001401574a0(_DAT_14e64f750 + 0x48);
    func_0x0001401574a0(lVar9 + 0x30);
    func_0x0001401574a0(lVar9 + 0x18);
    func_0x0001401574a0(lVar9);
    func_0x000146e9f3a0(lVar9,0x60);
  }
  _DAT_14e64f750 = 0;
  pplVar3 = (longlong **)(param_1 + 0x21);
  pplVar11 = (longlong **)**pplVar3;
  cVar5 = *(char *)((longlong)pplVar11 + 0x19);
  while (cVar5 == '\0') {
    plVar6 = pplVar11[5];
    if (plVar6 != (longlong *)0x0) {
      (**(code **)*plVar6)(plVar6,1);
    }
    pplVar8 = (longlong **)pplVar11[2];
    if (*(char *)((longlong)pplVar8 + 0x19) == '\0') {
      cVar5 = *(char *)((longlong)*pplVar8 + 0x19);
      pplVar11 = pplVar8;
      pplVar8 = (longlong **)*pplVar8;
      while (cVar5 == '\0') {
        cVar5 = *(char *)((longlong)*pplVar8 + 0x19);
        pplVar11 = pplVar8;
        pplVar8 = (longlong **)*pplVar8;
      }
    }
    else {
      cVar5 = *(char *)((longlong)pplVar11[1] + 0x19);
      pplVar10 = (longlong **)pplVar11[1];
      pplVar8 = pplVar11;
      while ((pplVar11 = pplVar10, cVar5 == '\0' && (pplVar8 == (longlong **)pplVar11[2]))) {
        cVar5 = *(char *)((longlong)pplVar11[1] + 0x19);
        pplVar10 = (longlong **)pplVar11[1];
        pplVar8 = pplVar11;
      }
    }
    cVar5 = *(char *)((longlong)pplVar11 + 0x19);
  }
  func_0x000146e9f800(param_1 + 0x29);
  plVar6 = (longlong *)param_1[0x25];
  if (plVar6 != (longlong *)0x0) {
    LOCK();
    plVar1 = plVar6 + 1;
    iVar4 = *(int *)plVar1;
    *(int *)plVar1 = *(int *)plVar1 + -1;
    UNLOCK();
    if (iVar4 == 1) {
      (**(code **)*plVar6)(plVar6);
      LOCK();
      piVar2 = (int *)((longlong)plVar6 + 0xc);
      iVar4 = *piVar2;
      *piVar2 = *piVar2 + -1;
      UNLOCK();
      if (iVar4 == 1) {
        (**(code **)(*plVar6 + 8))(plVar6);
      }
    }
  }
  func_0x00014014ebd0(pplVar3,pplVar3,(*pplVar3)[1]);
  func_0x000146e9f3a0(*pplVar3,0x30);
  func_0x00014017ac10(param_1 + 0x1e);
  plVar6 = param_1 + 8;
  func_0x000140154d90(plVar6,plVar6,*(undefined8 *)(*plVar6 + 8));
  func_0x000146e9f3a0(*plVar6,0x48);
  param_1[1] = &UNK_14b1c0ba0;
  if (param_1[2] != 0) {
    func_0x000146f3b560();
  }
  if (param_1[3] != 0) {
    func_0x000146f3b560();
  }
  if (param_1[5] != 0) {
    func_0x000146f3b560();
  }
  if (param_1[4] != 0) {
    func_0x000146f3b560();
  }
  plVar6 = (longlong *)param_1[6];
  if ((plVar6 != (longlong *)0x0) &&
     (*(int *)(plVar6 + 1) = *(int *)(plVar6 + 1) + -1, *(int *)(plVar6 + 1) < 1)) {
    (**(code **)(*plVar6 + 8))();
  }
  return;
}



// FUNCTION 141c26340 FUN_141c26340

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c26340(longlong *param_1,undefined8 param_2,undefined8 param_3)

{
  char cVar1;
  int iVar2;
  code *pcVar3;
  longlong **pplVar4;
  undefined4 uVar5;
  longlong *plVar6;
  undefined8 *puVar7;
  longlong **pplVar8;
  longlong **pplVar9;
  longlong lVar10;
  longlong **pplVar11;
  undefined auStack_d8 [32];
  int aiStack_b8 [2];
  undefined8 uStack_b0;
  undefined4 uStack_a8;
  int iStack_a4;
  int iStack_a0;
  undefined4 uStack_8c;
  int iStack_88;
  int iStack_84;
  int iStack_80;
  int iStack_7c;
  undefined4 uStack_70;
  undefined4 uStack_6c;
  undefined8 uStack_68;
  ulonglong auStack_60 [2];
  undefined8 uStack_50;
  ulonglong uStack_48;
  ulonglong uStack_40;
  
  uStack_68 = 0xfffffffffffffffe;
  uStack_40 = _DAT_14dca9408 ^ (ulonglong)auStack_d8;
  uStack_b0 = param_2;
  plVar6 = (longlong *)func_0x0001450be2c0(param_3);
  if (plVar6 != (longlong *)0x0) {
    uVar5 = (**(code **)(*plVar6 + 0x12e0))(plVar6);
    puVar7 = (undefined8 *)func_0x0001470bc7d0(uVar5);
    pplVar9 = *(longlong ***)*puVar7;
    while (*(char *)((longlong)pplVar9 + 0x19) == '\0') {
      pplVar8 = (longlong **)*pplVar9[5];
      cVar1 = *(char *)((longlong)pplVar8 + 0x19);
      while (cVar1 == '\0') {
        iVar2 = *(int *)(pplVar9 + 4);
        pplVar11 = pplVar8 + 5;
        uVar5 = *(undefined4 *)(pplVar8 + 4);
        func_0x0001471ecc90(&uStack_a8);
        uStack_6c = 0x2c;
        uStack_a8 = (**(code **)(*plVar6 + 0x12e0))(plVar6);
        aiStack_b8[0] = iVar2;
        func_0x000146e922e0(aiStack_b8,&iStack_a4);
        iStack_a0 = aiStack_b8[0] + 0xc4 + iStack_a4;
        if ((longlong *)0x7 < pplVar8[8]) {
          pplVar11 = (longlong **)*pplVar11;
        }
        uStack_50 = 0;
        uStack_48 = 7;
        auStack_60[0] = 0;
        lVar10 = -1;
        do {
          lVar10 = lVar10 + 1;
        } while (*(short *)((longlong)pplVar11 + lVar10 * 2) != 0);
        uStack_8c = uVar5;
        func_0x00014014c8d0(auStack_60,pplVar11);
        uStack_70 = func_0x0001470bc640(auStack_60);
        if (7 < uStack_48) {
          if (0xfff < uStack_48 * 2 + 2) {
            if (0x1f < (auStack_60[0] - *(longlong *)(auStack_60[0] - 8)) - 8) {
              func_0x000148aaf304(*(longlong *)(auStack_60[0] - 8),uStack_48 * 2 + 0x29);
              pcVar3 = (code *)swi(3);
              (*pcVar3)();
              return;
            }
          }
          func_0x000146e9f3a0();
        }
        uStack_50 = 0;
        uStack_48 = 7;
        auStack_60[0] = auStack_60[0] & 0xffffffffffff0000;
        aiStack_b8[0] = *(int *)(*param_1 + 0x108);
        func_0x000146e922e0(aiStack_b8,&iStack_80);
        iStack_7c = aiStack_b8[0] + iStack_80 + 0xc4;
        aiStack_b8[0] = 1;
        func_0x000146e922e0(aiStack_b8,&iStack_88);
        iStack_84 = aiStack_b8[0] + iStack_88 + 0xc4;
        func_0x0001454ca650(uStack_b0,&uStack_a8);
        pplVar11 = (longlong **)pplVar8[2];
        if (*(char *)((longlong)pplVar11 + 0x19) == '\0') {
          cVar1 = *(char *)((longlong)*pplVar11 + 0x19);
          pplVar8 = pplVar11;
          pplVar11 = (longlong **)*pplVar11;
          while (cVar1 == '\0') {
            cVar1 = *(char *)((longlong)*pplVar11 + 0x19);
            pplVar8 = pplVar11;
            pplVar11 = (longlong **)*pplVar11;
          }
        }
        else {
          cVar1 = *(char *)((longlong)pplVar8[1] + 0x19);
          pplVar4 = (longlong **)pplVar8[1];
          pplVar11 = pplVar8;
          while ((pplVar8 = pplVar4, cVar1 == '\0' && (pplVar11 == (longlong **)pplVar8[2]))) {
            cVar1 = *(char *)((longlong)pplVar8[1] + 0x19);
            pplVar4 = (longlong **)pplVar8[1];
            pplVar11 = pplVar8;
          }
        }
        cVar1 = *(char *)((longlong)pplVar8 + 0x19);
      }
      pplVar8 = (longlong **)pplVar9[2];
      if (*(char *)((longlong)pplVar8 + 0x19) == '\0') {
        cVar1 = *(char *)((longlong)*pplVar8 + 0x19);
        pplVar9 = pplVar8;
        pplVar8 = (longlong **)*pplVar8;
        while (cVar1 == '\0') {
          cVar1 = *(char *)((longlong)*pplVar8 + 0x19);
          pplVar9 = pplVar8;
          pplVar8 = (longlong **)*pplVar8;
        }
      }
      else {
        cVar1 = *(char *)((longlong)pplVar9[1] + 0x19);
        pplVar11 = (longlong **)pplVar9[1];
        pplVar8 = pplVar9;
        while ((pplVar9 = pplVar11, cVar1 == '\0' && (pplVar8 == (longlong **)pplVar9[2]))) {
          cVar1 = *(char *)((longlong)pplVar9[1] + 0x19);
          pplVar11 = (longlong **)pplVar9[1];
          pplVar8 = pplVar9;
        }
      }
    }
  }
  func_0x000148860610(uStack_40 ^ (ulonglong)auStack_d8);
  return;
}



// FUNCTION 141c26d90 FUN_141c26d90

void FUN_141c26d90(undefined8 param_1,longlong **param_2)

{
  longlong *plVar1;
  char cVar2;
  
  plVar1 = *param_2;
  if (plVar1 != (longlong *)0x0) {
    cVar2 = func_0x000145f38210(plVar1,0);
    if (cVar2 != '\0') {
      (**(code **)(*plVar1 + 0x550))(plVar1,0x53,1);
      (**(code **)(*plVar1 + 0x550))(plVar1,0x54,1);
    }
  }
  return;
}



// FUNCTION 141c26f50 FUN_141c26f50

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c26f50(longlong param_1,undefined8 *param_2)

{
  undefined8 *puVar1;
  longlong lVar2;
  undefined4 uStackX_8;
  float fStackX_c;
  
  lVar2 = func_0x000148aa307c(*param_2,0,0x14dcb4468,0x14dd34318,0);
  if (lVar2 == 0) {
    return;
  }
  fStackX_c = (float)*(int *)(*(longlong *)(param_1 + 8) + 0x108) * _DAT_1491be9b0 + _DAT_1491ac5f8;
  uStackX_8 = 4;
  puVar1 = *(undefined8 **)(lVar2 + 0x510);
  if (puVar1 != *(undefined8 **)(lVar2 + 0x518)) {
    *puVar1 = CONCAT44(fStackX_c,4);
    *(longlong *)(lVar2 + 0x510) = *(longlong *)(lVar2 + 0x510) + 8;
    return;
  }
  func_0x000140179dd0(lVar2 + 0x508,puVar1,&uStackX_8);
  return;
}



// FUNCTION 141c26fc0 FUN_141c26fc0

void FUN_141c26fc0(undefined8 param_1,longlong **param_2)

{
  longlong *plVar1;
  char cVar2;
  
  plVar1 = *param_2;
  if (plVar1 != (longlong *)0x0) {
    cVar2 = func_0x000145f38210(plVar1,0);
    if (cVar2 != '\0') {
      (**(code **)(*plVar1 + 0x550))(plVar1,5,1);
      (**(code **)(*plVar1 + 0x550))(plVar1,0x32,1);
    }
  }
  return;
}



// FUNCTION 141c27040 FUN_141c27040

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c27040(longlong param_1,undefined8 *param_2)

{
  undefined8 *puVar1;
  longlong lVar2;
  undefined4 uStackX_8;
  float fStackX_c;
  
  lVar2 = func_0x000148aa307c(*param_2,0,0x14dcb4468,0x14dd34318,0);
  if (lVar2 == 0) {
    return;
  }
  fStackX_c = (float)*(int *)(*(longlong *)(param_1 + 8) + 0x108) * _DAT_1491be9b0 + _DAT_1491ac5f8;
  uStackX_8 = 7;
  puVar1 = *(undefined8 **)(lVar2 + 0x510);
  if (puVar1 != *(undefined8 **)(lVar2 + 0x518)) {
    *puVar1 = CONCAT44(fStackX_c,7);
    *(longlong *)(lVar2 + 0x510) = *(longlong *)(lVar2 + 0x510) + 8;
    return;
  }
  func_0x000140179dd0(lVar2 + 0x508,puVar1,&uStackX_8);
  return;
}



// FUNCTION 141c270b0 FUN_141c270b0

void FUN_141c270b0(undefined8 param_1,longlong **param_2)

{
  longlong *plVar1;
  char cVar2;
  
  plVar1 = *param_2;
  if (plVar1 != (longlong *)0x0) {
    cVar2 = func_0x000145f38210(plVar1,0);
    if (cVar2 != '\0') {
      (**(code **)(*plVar1 + 0x550))(plVar1,0x68,1);
    }
  }
  return;
}



// FUNCTION 141c27df0 FUN_141c27df0

undefined8 *
FUN_141c27df0(longlong param_1,undefined8 *param_2,undefined8 param_3,undefined8 param_4)

{
  undefined8 uVar1;
  longlong lVar2;
  undefined8 *puVar3;
  int iVar4;
  longlong lVar5;
  undefined auStack_18 [16];
  
  lVar2 = *(longlong *)(param_1 + 0x20);
  if ((ulonglong)(*(longlong *)(param_1 + 0x28) - lVar2 >> 5) < 3) {
    *param_2 = 0;
    param_2[2] = 0;
    param_2[3] = 7;
    *(undefined2 *)param_2 = 0;
    func_0x00014014c8d0(param_2,&DAT_14baf7f08,0);
    return param_2;
  }
  puVar3 = (undefined8 *)(lVar2 + 0x20);
  iVar4 = 0;
  if (-1 < *(int *)(param_1 + 0x108)) {
    iVar4 = *(int *)(param_1 + 0x108);
  }
  if (7 < *(ulonglong *)(lVar2 + 0x38)) {
    puVar3 = (undefined8 *)*puVar3;
  }
  uVar1 = func_0x000146e8cf20(auStack_18,puVar3,iVar4,param_4,0,0xfffffffffffffffe);
  lVar2 = func_0x00014014f430(uVar1);
  *param_2 = 0;
  param_2[2] = 0;
  param_2[3] = 7;
  *(undefined2 *)param_2 = 0;
  lVar5 = -1;
  do {
    lVar5 = lVar5 + 1;
  } while (*(short *)(lVar2 + lVar5 * 2) != 0);
  func_0x00014014c8d0(param_2,lVar2);
  func_0x000146e8c910(auStack_18);
  return param_2;
}


