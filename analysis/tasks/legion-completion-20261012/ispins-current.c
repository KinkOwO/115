// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 142530c30 FUN_142530c30

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_142530c30(void)

{
  longlong *plVar1;
  int *piVar2;
  int iVar3;
  undefined2 *puVar4;
  undefined8 *puVar5;
  longlong lVar6;
  longlong lVar7;
  longlong lVar8;
  undefined8 uVar9;
  longlong lVar10;
  undefined auStack_a8 [32];
  longlong lStack_88;
  undefined8 uStack_80;
  undefined auStack_78 [8];
  longlong *plStack_70;
  undefined2 uStack_68;
  undefined uStack_66;
  int iStack_65;
  undefined4 uStack_61;
  ulonglong uStack_5d;
  undefined8 auStack_55 [4];
  undefined4 uStack_35;
  undefined2 uStack_31;
  undefined auStack_2f [18];
  undefined uStack_1d;
  undefined2 uStack_1b;
  ulonglong uStack_18;
  
  uStack_80 = 0xfffffffffffffffe;
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_a8;
  uStack_68 = 0xffff;
  uStack_66 = 0xff;
  iStack_65 = 0xe;
  lVar8 = 4;
  uStack_61 = 4;
  uStack_5d = 0xffffffffffffffff;
  puVar5 = auStack_55;
  lVar6 = 4;
  do {
    *puVar5 = 0xffffffffffffffff;
    puVar5 = puVar5 + 1;
    lVar6 = lVar6 + -1;
  } while (lVar6 != 0);
  lVar10 = 3;
  lVar7 = 3;
  puVar4 = &uStack_31;
  lVar6 = 0;
  do {
    *(undefined4 *)(puVar4 + -2) = 0;
    *puVar4 = 0;
    *(undefined *)(puVar4 + 1) = 0;
    puVar4 = puVar4 + 4;
    lVar7 = lVar7 + -1;
  } while (lVar7 != 0);
  puVar5 = auStack_55;
  do {
    *puVar5 = 0xffffffffffffffff;
    puVar5 = puVar5 + 1;
    lVar8 = lVar8 + -1;
  } while (lVar8 != 0);
  puVar4 = &uStack_31;
  do {
    *(undefined4 *)(puVar4 + -2) = 0;
    *puVar4 = 0;
    *(undefined *)(puVar4 + 1) = 0;
    puVar4 = puVar4 + 4;
    lVar10 = lVar10 + -1;
  } while (lVar10 != 0);
  uStack_1d = 0;
  uStack_1b = 0;
  func_0x000146ea0be0(&uStack_68,0x4f);
  if (_DAT_14e639fc8 == 0) {
    lStack_88 = func_0x000146e8ba20(0x1c0);
    lVar8 = lVar6;
    if (lStack_88 != 0) {
      lVar8 = func_0x000142527f70(lStack_88);
    }
    _DAT_14e639fc8 = lVar8;
    (***(code ***)(lVar8 + 0x50))();
  }
  func_0x00014252a840(_DAT_14e639fc8,&uStack_68);
  func_0x000142ac2220(_DAT_14e683c40,uStack_61);
  func_0x000142ac2290(_DAT_14e683c40,iStack_65);
  func_0x000142ac21b0(_DAT_14e683c40,uStack_5d & 0xffffffff);
  if (iStack_65 == 6) {
    if (_DAT_14e639fc8 == 0) {
      lStack_88 = func_0x000146e8ba20(0x1c0);
      lVar8 = lVar6;
      if (lStack_88 != 0) {
        lVar8 = func_0x000142527f70(lStack_88);
      }
      _DAT_14e639fc8 = lVar8;
      (***(code ***)(lVar8 + 0x50))();
    }
    uVar9 = 0;
    lVar8 = _DAT_14e639fc8;
  }
  else {
    if (iStack_65 != 2) goto LAB_142530e06;
    lVar8 = func_0x000141efe520();
    uVar9 = 2;
  }
  func_0x00014106e400(lVar8,uVar9);
LAB_142530e06:
  if (_DAT_14e639fc8 == 0) {
    lStack_88 = func_0x000146e8ba20(0x1c0);
    if (lStack_88 != 0) {
      lVar6 = func_0x000142527f70(lStack_88);
    }
    _DAT_14e639fc8 = lVar6;
    (***(code ***)(lVar6 + 0x50))();
  }
  puVar5 = (undefined8 *)func_0x000142528af0(_DAT_14e639fc8,auStack_78);
  func_0x000142525a00(*puVar5);
  if (plStack_70 != (longlong *)0x0) {
    LOCK();
    plVar1 = plStack_70 + 1;
    iVar3 = *(int *)plVar1;
    *(int *)plVar1 = *(int *)plVar1 + -1;
    UNLOCK();
    if (iVar3 == 1) {
      (**(code **)*plStack_70)(plStack_70);
      LOCK();
      piVar2 = (int *)((longlong)plStack_70 + 0xc);
      iVar3 = *piVar2;
      *piVar2 = *piVar2 + -1;
      UNLOCK();
      if (iVar3 == 1) {
        (**(code **)(*plStack_70 + 8))(plStack_70);
      }
    }
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_a8);
  return;
}



// FUNCTION 142530eb0 FUN_142530eb0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_142530eb0(undefined8 param_1,undefined8 param_2,undefined8 param_3,undefined8 param_4)

{
  longlong lVar1;
  short sStackX_18;
  undefined uStackX_1a;
  longlong lStackX_20;
  
  lVar1 = 0;
  sStackX_18 = 0;
  uStackX_1a = 0;
  func_0x000146ea0be0(&sStackX_18,3,param_3,param_4,0xfffffffffffffffe);
  if (_DAT_14e639fc8 == 0) {
    lStackX_20 = func_0x000146e8ba20(0x1c0);
    if (lStackX_20 != 0) {
      lVar1 = func_0x000142527f70(lStackX_20);
    }
    _DAT_14e639fc8 = lVar1;
    (***(code ***)(lVar1 + 0x50))();
  }
  FUN_14252c460(_DAT_14e639fc8,(int)sStackX_18,uStackX_1a);
  return;
}



// FUNCTION 14252c240 FUN_14252c240

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14252c240(longlong param_1,uint param_2)

{
  char cVar1;
  undefined8 *puVar2;
  undefined8 *puVar3;
  short sVar4;
  undefined8 *puVar5;
  undefined8 *puVar6;
  longlong *plVar7;
  undefined auStack_d8 [32];
  undefined8 uStack_b8;
  longlong *plStack_b0;
  longlong *plStack_a8;
  undefined auStack_98 [16];
  int iStack_88;
  undefined auStack_80 [64];
  longlong alStack_40 [2];
  longlong alStack_30 [3];
  ulonglong uStack_18;
  
  uStack_b8 = 0xfffffffffffffffe;
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_d8;
  if (param_2 == 0xffffffff) {
    param_2 = (uint)*(ushort *)(param_1 + 0x5a);
  }
  else {
    *(short *)(param_1 + 0x5a) = (short)param_2;
  }
  if ((short)param_2 != -1) {
    if (*(longlong *)(param_1 + 0xc0) != 0) {
      func_0x0001425307e0();
    }
    *(undefined8 *)(param_1 + 0xc0) = 0;
    func_0x000142516620(auStack_98);
    func_0x000147abf0f0(*(undefined8 *)(param_1 + 0xb0),*(undefined2 *)(param_1 + 0x5a),auStack_98);
    puVar2 = *(undefined8 **)(param_1 + 0xd8);
    cVar1 = *(char *)((longlong)(undefined8 *)puVar2[1] + 0x19);
    puVar3 = puVar2;
    puVar6 = (undefined8 *)puVar2[1];
    while (cVar1 == '\0') {
      if (*(int *)(puVar6 + 4) < iStack_88) {
        puVar5 = (undefined8 *)puVar6[2];
        puVar6 = puVar3;
      }
      else {
        puVar5 = (undefined8 *)*puVar6;
      }
      puVar3 = puVar6;
      puVar6 = puVar5;
      cVar1 = *(char *)((longlong)puVar5 + 0x19);
    }
    if (((*(char *)((longlong)puVar3 + 0x19) == '\0') && (*(int *)(puVar3 + 4) <= iStack_88)) &&
       (puVar3 != puVar2)) {
      plVar7 = (longlong *)puVar3[5];
      *(longlong **)(param_1 + 0xc0) = plVar7;
      sVar4 = *(short *)(param_1 + 0x5a);
      if (*(short *)(param_1 + 0x93) == sVar4) {
        *(int *)(plVar7 + 2) = (int)*(short *)(param_1 + 0x95);
        *(undefined8 *)(*(longlong *)(param_1 + 0xc0) + 0xd8) = *(undefined8 *)(param_1 + 0x93);
        sVar4 = *(short *)(param_1 + 0x5a);
        plVar7 = *(longlong **)(param_1 + 0xc0);
      }
      if (*(short *)(param_1 + 0x9b) == sVar4) {
        *(int *)(plVar7 + 2) = (int)*(short *)(param_1 + 0x9d);
        *(undefined8 *)(*(longlong *)(param_1 + 0xc0) + 0xd8) = *(undefined8 *)(param_1 + 0x9b);
        sVar4 = *(short *)(param_1 + 0x5a);
        plVar7 = *(longlong **)(param_1 + 0xc0);
      }
      if (*(short *)(param_1 + 0xa3) == sVar4) {
        *(int *)(plVar7 + 2) = (int)*(short *)(param_1 + 0xa5);
        *(undefined8 *)(*(longlong *)(param_1 + 0xc0) + 0xd8) = *(undefined8 *)(param_1 + 0xa3);
        plVar7 = *(longlong **)(param_1 + 0xc0);
      }
      (**(code **)(*plVar7 + 0x48))();
    }
    plStack_b0 = alStack_30;
    func_0x00014207fcd0(alStack_30,alStack_30,*(undefined8 *)(alStack_30[0] + 8));
    func_0x000146e9f3a0(alStack_30[0],0x40);
    plStack_a8 = alStack_40;
    func_0x00014207fcd0(alStack_40,alStack_40,*(undefined8 *)(alStack_40[0] + 8));
    func_0x000146e9f3a0(alStack_40[0],0x40);
    func_0x000140162230(auStack_80);
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_d8);
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



// FUNCTION 14252c460 FUN_14252c460

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14252c460(longlong param_1,int param_2,char param_3)

{
  short sVar1;
  undefined8 *puVar2;
  longlong lVar3;
  undefined8 uVar4;
  undefined8 uVar5;
  undefined8 uVar6;
  
  lVar3 = *(longlong *)(param_1 + 0xc0);
  if (lVar3 != 0) {
    if (*(short *)(lVar3 + 0xd8) == param_2) {
      *(bool *)(lVar3 + 9) = param_3 != '\x01';
    }
    else {
      if (_DAT_14e6343d0 == (undefined8 *)0x0) {
        lVar3 = func_0x000146e8ba20();
        if (lVar3 == 0) {
          _DAT_14e6343d0 = (undefined8 *)0x0;
        }
        else {
          _DAT_14e6343d0 = (undefined8 *)func_0x000146e93360(lVar3);
        }
        (**(code **)*_DAT_14e6343d0)();
        lVar3 = *(longlong *)(param_1 + 0xc0);
      }
      puVar2 = _DAT_14e6343d0;
      sVar1 = *(short *)(lVar3 + 0xd8);
      uVar4 = func_0x000146e8c7d0(&UNK_14982b970);
      uVar5 = func_0x000146e8c7d0(&UNK_1499da890);
      uVar6 = func_0x000146e8c7d0(&UNK_1499da8e0);
      func_0x000146e938e0(puVar2,0,uVar6,uVar5,0x332,0x14e652d20,uVar4,(int)sVar1,param_2);
    }
  }
  return;
}


