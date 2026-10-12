// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141c28a10 FUN_141c28a10

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c28a10(longlong param_1)

{
  undefined4 uVar1;
  longlong **pplVar2;
  longlong *plVar3;
  longlong *plVar4;
  code *pcVar5;
  undefined8 uVar6;
  undefined8 uVar7;
  longlong lVar8;
  longlong *****ppppplVar9;
  ulonglong uVar10;
  longlong **pplVar11;
  longlong lVar12;
  longlong **pplVar13;
  undefined auStack_198 [32];
  undefined *puStack_178;
  undefined8 uStack_170;
  longlong *plStack_168;
  undefined4 uStack_160;
  undefined4 uStack_15c;
  longlong *plStack_158;
  ulonglong uStack_150;
  undefined8 auStack_148 [2];
  undefined8 uStack_138;
  undefined8 uStack_130;
  undefined8 uStack_128;
  longlong **pplStack_120;
  longlong **pplStack_118;
  undefined8 *puStack_110;
  undefined auStack_108 [16];
  longlong *****ppppplStack_f8;
  undefined4 uStack_f0;
  undefined4 uStack_ec;
  longlong *plStack_e8;
  ulonglong uStack_e0;
  undefined auStack_d8 [160];
  ulonglong uStack_38;
  
  uStack_128 = 0xfffffffffffffffe;
  uStack_38 = _DAT_14dca9408 ^ (ulonglong)auStack_198;
  pplVar11 = *(longlong ***)(param_1 + 0x110);
  pplVar2 = *(longlong ***)(param_1 + 0x118);
  do {
    if (pplVar11 == pplVar2) {
      func_0x000148860610(uStack_38 ^ (ulonglong)auStack_198);
      return;
    }
    func_0x000147680d10(auStack_d8);
    ppppplStack_f8 = (longlong *****)0x0;
    plStack_e8 = (longlong *)0x0;
    uStack_e0 = 0;
    plVar3 = pplVar11[2];
    pplVar13 = pplVar11;
    if ((longlong *)0x7 < pplVar11[3]) {
      pplVar13 = (longlong **)*pplVar11;
    }
    if (plVar3 < (longlong *)0x8) {
      ppppplStack_f8 = (longlong *****)*pplVar13;
      uStack_f0 = *(undefined4 *)(pplVar13 + 1);
      uStack_ec = *(undefined4 *)((longlong)pplVar13 + 0xc);
      uStack_e0 = 7;
    }
    else {
      uVar10 = (ulonglong)plVar3 | 7;
      if (0x7ffffffffffffffe < uVar10) {
        uVar10 = 0x7ffffffffffffffe;
      }
      ppppplStack_f8 = (longlong *****)func_0x00014014cb50(&ppppplStack_f8,uVar10 + 1);
      func_0x000148aa1e60(ppppplStack_f8,pplVar13,(longlong)plVar3 * 2 + 2);
      uStack_e0 = uVar10;
    }
    pplStack_120 = &plStack_168;
    plStack_168 = (longlong *)0x0;
    plStack_158 = (longlong *)0x0;
    uStack_150 = 0;
    plVar4 = pplVar11[2];
    pplVar13 = pplVar11;
    if ((longlong *)0x7 < pplVar11[3]) {
      pplVar13 = (longlong **)*pplVar11;
    }
    plStack_e8 = plVar3;
    if (plVar4 < (longlong *)0x8) {
      plStack_168 = *pplVar13;
      uStack_160 = *(undefined4 *)(pplVar13 + 1);
      uStack_15c = *(undefined4 *)((longlong)pplVar13 + 0xc);
      uStack_150 = 7;
    }
    else {
      uVar10 = (ulonglong)plVar4 | 7;
      if (0x7ffffffffffffffe < uVar10) {
        uVar10 = 0x7ffffffffffffffe;
      }
      plStack_168 = (longlong *)func_0x00014014cb50(&plStack_168,uVar10 + 1);
      func_0x000148aa1e60(plStack_168,pplVar13,(longlong)plVar4 * 2 + 2);
      uStack_150 = uVar10;
    }
    plStack_158 = plVar4;
    func_0x000147684a60(&plStack_168,&ppppplStack_f8);
    uVar6 = func_0x0001459a9800(_DAT_14e66c090);
    ppppplVar9 = (longlong *****)&ppppplStack_f8;
    if (7 < uStack_e0) {
      ppppplVar9 = ppppplStack_f8;
    }
    puStack_178 = auStack_d8;
    func_0x0001428b2f80(uVar6,0,0,ppppplVar9);
    uVar6 = func_0x0001459a9800(_DAT_14e66c090);
    pplStack_118 = &plStack_168;
    uVar1 = *(undefined4 *)(pplVar11 + 4);
    uVar7 = func_0x000146e8c7d0(&UNK_1491ce3a0);
    uVar7 = func_0x000146e8cf20(auStack_108,uVar7,uVar1);
    lVar8 = func_0x00014014f430(uVar7);
    plStack_168 = (longlong *)0x0;
    plStack_158 = (longlong *)0x0;
    uStack_150 = 7;
    lVar12 = -1;
    do {
      lVar12 = lVar12 + 1;
    } while (*(short *)(lVar8 + lVar12 * 2) != 0);
    func_0x00014014c8d0(&plStack_168,lVar8);
    puStack_110 = auStack_148;
    ppppplVar9 = (longlong *****)&ppppplStack_f8;
    if (7 < uStack_e0) {
      ppppplVar9 = ppppplStack_f8;
    }
    auStack_148[0] = 0;
    uStack_138 = 0;
    uStack_130 = 7;
    lVar8 = -1;
    do {
      lVar8 = lVar8 + 1;
    } while (*(short *)((longlong)ppppplVar9 + lVar8 * 2) != 0);
    func_0x00014014c8d0(auStack_148);
    uStack_170 = 0;
    puStack_178 = (undefined *)0x0;
    func_0x0001428b9850(uVar6,0,auStack_148,&plStack_168);
    func_0x000146e8c910(auStack_108);
    if (7 < uStack_e0) {
      if (0xfff < uStack_e0 * 2 + 2) {
        if (0x1f < (ulonglong)((longlong)ppppplStack_f8 + (-8 - (longlong)ppppplStack_f8[-1]))) {
          func_0x000148aaf304(ppppplStack_f8[-1],uStack_e0 * 2 + 0x29);
          pcVar5 = (code *)swi(3);
          (*pcVar5)();
          return;
        }
      }
      func_0x000146e9f3a0();
    }
    plStack_e8 = (longlong *)0x0;
    uStack_e0 = 7;
    ppppplStack_f8 = (longlong *****)((ulonglong)ppppplStack_f8 & 0xffffffffffff0000);
    func_0x000147681700(auStack_d8);
    pplVar11 = pplVar11 + 5;
  } while( true );
}



// FUNCTION 141c28d30 FUN_141c28d30

undefined2 * FUN_141c28d30(longlong param_1,undefined2 *param_2)

{
  longlong **pplVar1;
  char cVar2;
  longlong *plVar3;
  longlong **pplVar4;
  longlong **pplVar5;
  longlong lVar6;
  longlong **pplVar7;
  longlong lVar8;
  undefined uStackX_8;
  
  *(undefined2 *)(param_1 + 8) = *param_2;
  *(undefined2 *)(param_1 + 10) = param_2[1];
  *(undefined2 *)(param_1 + 0xc) = param_2[2];
  *(undefined4 *)(param_1 + 0x10) = *(undefined4 *)(param_2 + 4);
  *(undefined4 *)(param_1 + 0x14) = *(undefined4 *)(param_2 + 6);
  *(undefined *)(param_1 + 0x18) = *(undefined *)(param_2 + 8);
  *(undefined *)(param_1 + 0x19) = *(undefined *)((longlong)param_2 + 0x11);
  if ((undefined8 *)(param_1 + 0x20) != (undefined8 *)(param_2 + 0xc)) {
    func_0x00014024c7d0((undefined8 *)(param_1 + 0x20),*(undefined8 *)(param_2 + 0xc),
                        *(undefined8 *)(param_2 + 0x10),uStackX_8);
  }
  *(undefined8 *)(param_1 + 0x38) = *(undefined8 *)(param_2 + 0x18);
  *(undefined8 *)(param_1 + 0x40) = *(undefined8 *)(param_2 + 0x1c);
  *(undefined8 *)(param_1 + 0x48) = *(undefined8 *)(param_2 + 0x20);
  *(undefined8 *)(param_1 + 0x50) = *(undefined8 *)(param_2 + 0x24);
  *(undefined8 *)(param_1 + 0x58) = *(undefined8 *)(param_2 + 0x28);
  func_0x00014069cda0(param_1 + 0x60,param_2 + 0x2c);
  func_0x00014069cda0(param_1 + 0x70,param_2 + 0x34);
  pplVar1 = (longlong **)(param_1 + 0x88);
  *(undefined4 *)(param_1 + 0x80) = *(undefined4 *)(param_2 + 0x3c);
  if (pplVar1 != (longlong **)(param_2 + 0x40)) {
    plVar3 = *pplVar1;
    func_0x000140150890(pplVar1,pplVar1,plVar3[1]);
    plVar3[1] = (longlong)plVar3;
    *plVar3 = (longlong)plVar3;
    plVar3[2] = (longlong)plVar3;
    *(undefined8 *)(param_1 + 0x90) = 0;
    lVar8 = func_0x0001401dbaa0(pplVar1,(*(longlong **)(param_2 + 0x40))[1],*pplVar1,uStackX_8);
    (*pplVar1)[1] = lVar8;
    pplVar4 = (longlong **)*pplVar1;
    *(undefined8 *)(param_1 + 0x90) = *(undefined8 *)(param_2 + 0x44);
    pplVar5 = (longlong **)pplVar4[1];
    if (*(char *)((longlong)pplVar5 + 0x19) == '\0') {
      cVar2 = *(char *)((longlong)*pplVar5 + 0x19);
      pplVar7 = (longlong **)*pplVar5;
      while (cVar2 == '\0') {
        cVar2 = *(char *)((longlong)*pplVar7 + 0x19);
        pplVar5 = pplVar7;
        pplVar7 = (longlong **)*pplVar7;
      }
      *pplVar4 = (longlong *)pplVar5;
      lVar8 = (*pplVar1)[1];
      lVar6 = *(longlong *)(lVar8 + 0x10);
      cVar2 = *(char *)(lVar6 + 0x19);
      while (cVar2 == '\0') {
        cVar2 = *(char *)(*(longlong *)(lVar6 + 0x10) + 0x19);
        lVar8 = lVar6;
        lVar6 = *(longlong *)(lVar6 + 0x10);
      }
      (*pplVar1)[2] = lVar8;
    }
    else {
      *pplVar4 = (longlong *)pplVar4;
      (*pplVar1)[2] = (longlong)*pplVar1;
    }
  }
  *(undefined4 *)(param_1 + 0x98) = *(undefined4 *)(param_2 + 0x48);
  if ((undefined8 *)(param_1 + 0xa0) != (undefined8 *)(param_2 + 0x4c)) {
    func_0x00014024c190((undefined8 *)(param_1 + 0xa0),*(undefined8 *)(param_2 + 0x4c),
                        *(undefined8 *)(param_2 + 0x50),uStackX_8);
  }
  func_0x00014069cda0(param_1 + 0xb8,param_2 + 0x58);
  return (undefined2 *)(param_1 + 8);
}



// FUNCTION 141c279f0 FUN_141c279f0

undefined8 * FUN_141c279f0(longlong param_1,undefined8 *param_2)

{
  code *pcVar1;
  undefined8 uVar2;
  longlong lVar3;
  longlong lVar4;
  undefined8 *puVar5;
  undefined auStack_18 [16];
  
  lVar3 = *(longlong *)(param_1 + 0x20);
  if ((ulonglong)(*(longlong *)(param_1 + 0x28) - lVar3 >> 5) < 3) {
    *param_2 = 0;
    param_2[2] = 0;
    param_2[3] = 7;
    *(undefined2 *)param_2 = 0;
    func_0x00014014c8d0(param_2,&DAT_14baf7f08,0);
    return param_2;
  }
  puVar5 = (undefined8 *)(lVar3 + 0x20);
  lVar4 = *(longlong *)(param_1 + 0x110);
  if (1 < (ulonglong)((*(longlong *)(param_1 + 0x118) - lVar4) / 0x28)) {
    if (7 < *(ulonglong *)(lVar3 + 0x38)) {
      puVar5 = (undefined8 *)*puVar5;
    }
    uVar2 = func_0x000146e8cf20(auStack_18,puVar5,*(undefined4 *)(lVar4 + 0x20),
                                *(undefined4 *)(lVar4 + 0x48),0,0xfffffffffffffffe);
    lVar3 = func_0x00014014f430(uVar2);
    *param_2 = 0;
    param_2[2] = 0;
    param_2[3] = 7;
    *(undefined2 *)param_2 = 0;
    lVar4 = -1;
    do {
      lVar4 = lVar4 + 1;
    } while (*(short *)(lVar3 + lVar4 * 2) != 0);
    func_0x00014014c8d0(param_2,lVar3);
    func_0x000146e8c910(auStack_18);
    return param_2;
  }
  func_0x0001401790b0();
  pcVar1 = (code *)swi(3);
  puVar5 = (undefined8 *)(*pcVar1)();
  return puVar5;
}



// FUNCTION 141c27ef0 FUN_141c27ef0

undefined8 *
FUN_141c27ef0(longlong param_1,undefined8 *param_2,undefined8 param_3,undefined8 param_4)

{
  if ((ulonglong)(*(longlong *)(param_1 + 0x28) - *(longlong *)(param_1 + 0x20) >> 5) < 3) {
    *param_2 = 0;
    param_2[2] = 0;
    param_2[3] = 7;
    *(undefined2 *)param_2 = 0;
    func_0x00014014c8d0(param_2,&DAT_14baf7f08);
  }
  else {
    func_0x00014014c810(param_2,*(longlong *)(param_1 + 0x20) + 0x40,0,param_4,0,0xfffffffffffffffe)
    ;
  }
  return param_2;
}


