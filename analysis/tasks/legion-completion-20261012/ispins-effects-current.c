// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 142530620 FUN_142530620

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_142530620(longlong *param_1)

{
  code *pcVar1;
  undefined8 uVar2;
  longlong lVar3;
  longlong lVar4;
  undefined8 *puVar5;
  longlong lVar6;
  undefined auStack_98 [32];
  undefined4 uStack_78;
  undefined8 auStack_68 [2];
  undefined8 uStack_58;
  undefined8 uStack_50;
  undefined8 *puStack_48;
  undefined8 uStack_40;
  undefined2 uStack_38;
  undefined6 uStack_36;
  undefined8 uStack_28;
  ulonglong uStack_20;
  ulonglong uStack_18;
  
  uStack_40 = 0xfffffffffffffffe;
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_98;
  if (*(char *)(param_1 + 1) != '\x01') {
    uVar2 = func_0x00014667eb40(_DAT_14e683c78,0x285);
    uStack_78 = 0;
    lVar3 = func_0x000148aa307c(uVar2,0,0x14dcb4760,0x14e127538);
    if (lVar3 != 0) {
      puStack_48 = auStack_68;
      if (*(char *)((longlong)param_1 + 9) == '\x01') {
        lVar4 = func_0x00014723c170(0x5f738ff);
        auStack_68[0] = 0;
        uStack_58 = 0;
        uStack_50 = 7;
        lVar6 = -1;
        do {
          lVar6 = lVar6 + 1;
        } while (*(short *)(lVar4 + lVar6 * 2) != 0);
        func_0x00014014c8d0(auStack_68,lVar4);
        func_0x0001424ec580(lVar3,auStack_68,1);
      }
      else {
        puVar5 = (undefined8 *)(**(code **)(*param_1 + 0x58))(param_1,&uStack_38);
        if (7 < (ulonglong)puVar5[3]) {
          puVar5 = (undefined8 *)*puVar5;
        }
        auStack_68[0] = 0;
        uStack_58 = 0;
        uStack_50 = 7;
        lVar4 = -1;
        do {
          lVar4 = lVar4 + 1;
        } while (*(short *)((longlong)puVar5 + lVar4 * 2) != 0);
        func_0x00014014c8d0(auStack_68,puVar5);
        func_0x0001424ec580(lVar3,auStack_68,0);
        if (7 < uStack_20) {
          if (0xfff < uStack_20 * 2 + 2) {
            lVar3 = *(longlong *)(CONCAT62(uStack_36,uStack_38) + -8);
            if (0x1f < (CONCAT62(uStack_36,uStack_38) - lVar3) - 8U) {
              func_0x000148aaf304(lVar3,uStack_20 * 2 + 0x29);
              pcVar1 = (code *)swi(3);
              (*pcVar1)();
              return;
            }
          }
          func_0x000146e9f3a0();
        }
        uStack_28 = 0;
        uStack_20 = 7;
        uStack_38 = 0;
      }
    }
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_98);
  return;
}



// FUNCTION 142530430 FUN_142530430

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_142530430(longlong param_1)

{
  longlong lVar1;
  int iVar2;
  undefined8 uVar3;
  longlong lVar4;
  longlong lVar5;
  longlong *plVar6;
  longlong lVar7;
  undefined8 *puVar8;
  undefined auStack_108 [32];
  undefined4 uStack_e8;
  undefined auStack_d8 [16];
  undefined8 uStack_c8;
  undefined *puStack_c0;
  undefined auStack_b8 [32];
  undefined auStack_98 [24];
  longlong lStack_80;
  ulonglong uStack_18;
  
  uStack_c8 = 0xfffffffffffffffe;
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_108;
  FUN_142530620();
  uVar3 = func_0x00014667eb40(_DAT_14e683c78,0x285);
  uStack_e8 = 0;
  lVar4 = func_0x000148aa307c(uVar3,0,0x14dcb4760,0x14e127538);
  if (lVar4 != 0) {
    lVar5 = func_0x0001459a9080(_DAT_14e66c090);
    if (lVar5 != 0) {
      lVar5 = func_0x000140157c80(lVar5);
      if ((lVar5 != 0) && (*(char *)(param_1 + 9) != '\x01')) {
        func_0x000142516620(auStack_98);
        uVar3 = func_0x000141efe520();
        uVar3 = func_0x000142528b50(uVar3);
        func_0x000147abf0f0(uVar3,*(undefined2 *)(param_1 + 0xd8),auStack_98);
        if (*(char *)(param_1 + 8) == '\x01') {
          lVar5 = func_0x0001429deba0();
        }
        else {
          plVar6 = (longlong *)func_0x000142ab4570(_DAT_14e683c40,auStack_d8);
          iVar2 = func_0x000145a11a50();
          lVar5 = (longlong)iVar2 - *plVar6;
        }
        lVar1 = lVar5 % 0xe10 + SUB168(SEXT816(-0x7777777777777777) * SEXT816(lVar5 % 0xe10),8);
        lVar7 = SUB168(SEXT816(-0x7777777777777777) * SEXT816(lVar5),8) + lVar5;
        puStack_c0 = auStack_b8;
        puVar8 = (undefined8 *)(lStack_80 + 0x40);
        if (7 < *(ulonglong *)(lStack_80 + 0x58)) {
          puVar8 = (undefined8 *)*puVar8;
        }
        uVar3 = func_0x000146e8cf20(auStack_d8,puVar8,(lVar1 >> 5) - (lVar1 >> 0x3f) & 0xffffffff,
                                    (int)lVar5 + ((int)(lVar7 >> 5) - (int)(lVar7 >> 0x3f)) * -0x3c)
        ;
        uVar3 = func_0x00014014f430(uVar3);
        uVar3 = func_0x00014014c7b0(auStack_b8,uVar3);
        func_0x0001424ec580(lVar4,uVar3,0);
        func_0x000146e8c910(auStack_d8);
        func_0x000142516dd0(auStack_98);
      }
    }
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_108);
  return;
}



// FUNCTION 14252f820 FUN_14252f820

void FUN_14252f820(longlong param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  undefined8 *puVar1;
  undefined auStack_50 [56];
  undefined8 uStack_18;
  
  uStack_18 = 0;
  puVar1 = *(undefined8 **)(param_1 + 0x50);
  if (puVar1 != (undefined8 *)0x0) {
    uStack_18 = (**(code **)*puVar1)(puVar1,auStack_50,param_3,param_4,0xfffffffffffffffe);
  }
  FUN_14252e070(0x2eb,0x75d,0x111,auStack_50);
  return;
}



// FUNCTION 14252f880 FUN_14252f880

void FUN_14252f880(longlong param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  undefined8 *puVar1;
  undefined auStack_50 [56];
  undefined8 uStack_18;
  
  uStack_18 = 0;
  puVar1 = *(undefined8 **)(param_1 + 0x50);
  if (puVar1 != (undefined8 *)0x0) {
    uStack_18 = (**(code **)*puVar1)(puVar1,auStack_50,param_3,param_4,0xfffffffffffffffe);
  }
  FUN_14252e070(0x2eb,0x74a,0x111,auStack_50);
  return;
}



// FUNCTION 14252fb20 FUN_14252fb20

void FUN_14252fb20(longlong param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  undefined8 *puVar1;
  undefined auStack_50 [56];
  undefined8 uStack_18;
  
  uStack_18 = 0;
  puVar1 = *(undefined8 **)(param_1 + 0x50);
  if (puVar1 != (undefined8 *)0x0) {
    uStack_18 = (**(code **)*puVar1)(puVar1,auStack_50,param_3,param_4,0xfffffffffffffffe);
  }
  FUN_14252e070(0x2eb,0x75e,0x111,auStack_50);
  return;
}



// FUNCTION 14252ff50 FUN_14252ff50

void FUN_14252ff50(longlong param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  undefined8 *puVar1;
  undefined auStack_50 [56];
  undefined8 uStack_18;
  
  uStack_18 = 0;
  puVar1 = *(undefined8 **)(param_1 + 0x50);
  if (puVar1 != (undefined8 *)0x0) {
    uStack_18 = (**(code **)*puVar1)(puVar1,auStack_50,param_3,param_4,0xfffffffffffffffe);
  }
  FUN_14252e070(100,0x74f,0x211,auStack_50);
  return;
}



// FUNCTION 142530070 FUN_142530070

void FUN_142530070(longlong param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  undefined8 *puVar1;
  undefined auStack_50 [56];
  undefined8 uStack_18;
  
  uStack_18 = 0;
  puVar1 = *(undefined8 **)(param_1 + 0x50);
  if (puVar1 != (undefined8 *)0x0) {
    uStack_18 = (**(code **)*puVar1)(puVar1,auStack_50,param_3,param_4,0xfffffffffffffffe);
  }
  FUN_14252e070(0x2ed,0x752,0x211,auStack_50);
  return;
}



// FUNCTION 14742be50 FUN_14742be50

ulonglong FUN_14742be50(longlong param_1,int param_2)

{
  int *piVar1;
  
  piVar1 = *(int **)(param_1 + 0x10);
  while( true ) {
    if (piVar1 == *(int **)(param_1 + 0x18)) {
      return (ulonglong)piVar1 & 0xffffffffffffff00;
    }
    if (*piVar1 == param_2) break;
    piVar1 = piVar1 + 1;
  }
  return CONCAT71((int7)((ulonglong)piVar1 >> 8),1);
}



// FUNCTION 1473a1280 FUN_1473a1280

undefined4 FUN_1473a1280(longlong *param_1,int param_2)

{
  char cVar1;
  undefined8 *puVar2;
  undefined8 *puVar3;
  undefined8 *puVar4;
  undefined8 *puVar5;
  
  puVar2 = (undefined8 *)*param_1;
  cVar1 = *(char *)((longlong)(undefined8 *)puVar2[1] + 0x19);
  puVar3 = puVar2;
  puVar5 = (undefined8 *)puVar2[1];
  while (cVar1 == '\0') {
    if (*(int *)((longlong)puVar5 + 0x1c) < param_2) {
      puVar4 = (undefined8 *)puVar5[2];
      puVar5 = puVar3;
    }
    else {
      puVar4 = (undefined8 *)*puVar5;
    }
    puVar3 = puVar5;
    puVar5 = puVar4;
    cVar1 = *(char *)((longlong)puVar4 + 0x19);
  }
  if (((*(char *)((longlong)puVar3 + 0x19) == '\0') &&
      (*(int *)((longlong)puVar3 + 0x1c) <= param_2)) && (puVar3 != puVar2)) {
    return *(undefined4 *)(puVar3 + 4);
  }
  return 0;
}



// FUNCTION 1452b55e0 FUN_1452b55e0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_1452b55e0(void)

{
  longlong lVar1;
  undefined8 uVar2;
  undefined4 auStackX_18 [4];
  
  auStackX_18[0] = 0;
  func_0x000146ea0ba0(auStackX_18);
  if (_DAT_14e683c08 != 0) {
    lVar1 = func_0x000145f0ba60();
    if (lVar1 != 0) {
      uVar2 = func_0x000145f0ba60(_DAT_14e683c08);
      func_0x000145f06c60(uVar2,auStackX_18[0]);
    }
  }
  return;
}



// FUNCTION 14252e070 FUN_14252e070

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14252e070(undefined4 param_1,undefined8 param_2,int param_3,longlong *param_4)

{
  longlong *plVar1;
  code *pcVar2;
  char cVar3;
  longlong lVar4;
  longlong *plVar5;
  undefined8 *puVar6;
  undefined8 uVar7;
  ulonglong uVar8;
  undefined8 *puVar9;
  uint uVar10;
  undefined8 *puVar11;
  undefined8 *puVar12;
  undefined8 uStack_88;
  undefined8 uStack_80;
  longlong lStack_78;
  undefined8 *puStack_70;
  undefined auStack_68 [16];
  longlong lStack_58;
  undefined auStack_50 [16];
  longlong lStack_40;
  
  uStack_80 = 0xfffffffffffffffe;
  lVar4 = func_0x0001459a9260(_DAT_14e66c090);
  if (lVar4 == 0) {
    plVar5 = (longlong *)param_4[7];
    if (plVar5 == (longlong *)0x0) {
      return;
    }
    (**(code **)(*plVar5 + 0x20))(plVar5,plVar5 != param_4);
  }
  else {
    auStack_68 = ZEXT816(0);
    puVar9 = (undefined8 *)0x0;
    lStack_58 = 0;
    func_0x000145df44e0(lVar4,auStack_68,param_3,0,0);
    puVar11 = puVar9;
    puVar12 = puVar9;
    if ((longlong)(auStack_68._8_8_ - auStack_68._0_8_) >> 3 != 0) {
      do {
        plVar5 = (longlong *)
                 func_0x0001450be260(*(undefined8 *)((longlong)puVar12 + auStack_68._0_8_));
        if (((plVar5 != (longlong *)0x0) &&
            (cVar3 = (**(code **)(*plVar5 + 0xc10))(plVar5,param_2,0,0xffffffff), cVar3 != '\x01'))
           && (lVar4 = (**(code **)(*plVar5 + 0xbe0))(plVar5), lVar4 != 0)) {
          auStack_50 = ZEXT816(0);
          lStack_40 = 0;
          func_0x000145f440a0(lVar4,auStack_50);
          if (auStack_50._0_8_ == auStack_50._8_8_) {
            if (_DAT_14e634220 == (undefined8 *)0x0) {
              lStack_78 = func_0x000146e8ba20(0x40);
              puVar6 = puVar9;
              if (lStack_78 != 0) {
                puVar6 = (undefined8 *)func_0x000144b6dc50(lStack_78);
              }
              _DAT_14e634220 = puVar6;
              (**(code **)*puVar6)();
            }
            uVar7 = func_0x000144b99b30(_DAT_14e634220,param_1);
            plVar1 = (longlong *)param_4[7];
            if (plVar1 != (longlong *)0x0) {
              uStack_88 = uVar7;
              (**(code **)(*plVar1 + 0x10))(plVar1,&uStack_88);
            }
            puStack_70 = &uStack_88;
            uStack_88 = 0;
            func_0x000145c01600(plVar5,uVar7,plVar5,0,&uStack_88,param_2,0xffffffff,0,0);
          }
          (**(code **)(*plVar5 + 0x12d0))(plVar5,0);
          if (param_3 == 0x211) {
            pcVar2 = *(code **)(*plVar5 + 3000);
            uVar7 = (**(code **)(*plVar5 + 0xbc8))(plVar5);
            (*pcVar2)(plVar5,uVar7,0,1);
          }
          if (auStack_50._0_8_ != 0) {
            uVar8 = (lStack_40 - auStack_50._0_8_ >> 3) * 8;
            if (0xfff < uVar8) {
              if (0x1f < (auStack_50._0_8_ - *(longlong *)(auStack_50._0_8_ + -8)) - 8U) {
                func_0x000148aaf304(*(longlong *)(auStack_50._0_8_ + -8),uVar8 + 0x27);
                goto LAB_14252e374;
              }
            }
            func_0x000146e9f3a0();
            auStack_50 = ZEXT816(0);
            lStack_40 = 0;
          }
        }
        uVar10 = (int)puVar11 + 1;
        puVar11 = (undefined8 *)(ulonglong)uVar10;
        puVar12 = puVar12 + 1;
      } while ((ulonglong)(longlong)(int)uVar10 <
               (ulonglong)((longlong)(auStack_68._8_8_ - auStack_68._0_8_) >> 3));
    }
    if (auStack_68._0_8_ != 0) {
      uVar8 = (lStack_58 - auStack_68._0_8_ >> 3) * 8;
      lVar4 = auStack_68._0_8_;
      if (0xfff < uVar8) {
        uVar8 = uVar8 + 0x27;
        lVar4 = *(longlong *)(auStack_68._0_8_ + -8);
        if (0x1f < (auStack_68._0_8_ - lVar4) - 8U) {
LAB_14252e374:
          func_0x000148aaf304();
          pcVar2 = (code *)swi(3);
          (*pcVar2)();
          return;
        }
      }
      func_0x000146e9f3a0(lVar4,uVar8);
      auStack_68 = ZEXT816(0);
      lStack_58 = 0;
    }
    plVar5 = (longlong *)param_4[7];
    if (plVar5 == (longlong *)0x0) {
      return;
    }
    (**(code **)(*plVar5 + 0x20))(plVar5,plVar5 != param_4);
  }
  param_4[7] = 0;
  return;
}


