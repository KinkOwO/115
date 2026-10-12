// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 1420a5390 FUN_1420a5390

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_1420a5390(undefined4 param_1,char param_2)

{
  longlong *plVar1;
  int *piVar2;
  int iVar3;
  char cVar4;
  short sVar5;
  longlong lVar6;
  undefined8 uVar7;
  undefined8 *puVar8;
  undefined4 auStackX_10 [2];
  longlong lStack_30;
  longlong *plStack_28;
  int iStack_20;
  short sStack_1c;
  char cStack_1a;
  undefined8 uStack_19;
  undefined auStack_11 [2];
  undefined2 uStack_f;
  
  auStackX_10[0] = param_1;
  func_0x0001403a7590(&DAT_14e652c80,auStackX_10);
  if (param_2 == '\0') {
    return;
  }
  lVar6 = 0;
  iStack_20 = 0;
  sStack_1c = -1;
  cStack_1a = '\0';
  uStack_19 = 0;
  auStack_11[0] = 0;
  uStack_f = 0;
  func_0x000146ea0be0(&iStack_20,0x13);
  if (_DAT_14e650e88 == 0) {
    lStack_30 = func_0x000146e8ba20(0x128);
    if (lStack_30 != 0) {
      lVar6 = func_0x00014209af10(lStack_30);
    }
    _DAT_14e650e88 = lVar6;
    (***(code ***)(lVar6 + 0x50))();
  }
  func_0x0001420a1ae0(_DAT_14e650e88,auStack_11);
  if (iStack_20 != 1) {
    if (iStack_20 == 2) {
      if (sStack_1c == -1) {
        return;
      }
      uVar7 = func_0x000142080c60();
      func_0x0001420a10f0(uVar7,sStack_1c);
      uVar7 = func_0x000142080c60();
      FUN_14209fbe0(uVar7,0xffffffff);
      uVar7 = func_0x000142080c60();
      func_0x00014106e400(uVar7,2);
      uVar7 = func_0x00014667eb40(_DAT_14e683c78,0x286);
      lVar6 = func_0x000148aa307c(uVar7,0,0x14dcb4760,0x14dd93cc8,0);
      if (lVar6 == 0) {
        return;
      }
      func_0x000142096090(lVar6,0);
      func_0x00014208c540(lVar6,(int)sStack_1c);
      *(undefined *)(lVar6 + 0xf68) = 0;
      return;
    }
    if (iStack_20 != 4) {
      return;
    }
  }
  if (iStack_20 == 4) {
    uVar7 = func_0x00014667eb40(_DAT_14e683c78,0x286);
    lVar6 = func_0x000148aa307c(uVar7,0,0x14dcb4760,0x14dd93cc8,0);
    if (lVar6 != 0) {
      func_0x00014208a150(lVar6);
      *(undefined *)(lVar6 + 0xf68) = 1;
    }
  }
  if (cStack_1a == '\x01') {
    uVar7 = func_0x000142080c60();
    func_0x00014106e400(uVar7,0);
    uVar7 = func_0x00014667eb40(_DAT_14e683c78,0x286);
    lVar6 = func_0x000148aa307c(uVar7,0,0x14dcb4760,0x14dd93cc8,0);
    if ((lVar6 != 0) && (func_0x000142096090(lVar6,cStack_1a), *(char *)(lVar6 + 0xf68) != '\0')) {
      uVar7 = func_0x000142080c60();
      FUN_14209fbe0(uVar7,0xffffffff);
      uVar7 = func_0x000142080c60();
      func_0x00014106e400(uVar7,2);
      uVar7 = func_0x00014667eb40(_DAT_14e683c78,0x286);
      lVar6 = func_0x000148aa307c(uVar7,0,0x14dcb4760,0x14dd93cc8,0);
      if (lVar6 != 0) {
        uVar7 = func_0x000142080c60();
        sVar5 = func_0x00014209cf60(uVar7);
        func_0x00014208c540(lVar6,(int)sVar5);
        *(undefined *)(lVar6 + 0xf68) = 0;
        return;
      }
    }
  }
  else {
    uVar7 = func_0x000142080c60();
    func_0x00014106e400(uVar7,1);
    uVar7 = func_0x00014667eb40(_DAT_14e683c78,0x286);
    lVar6 = func_0x000148aa307c(uVar7,0,0x14dcb4760,0x14dd93cc8,0);
    if (lVar6 != 0) {
      func_0x000142096090(lVar6,0);
      func_0x00014208e290(lVar6);
      func_0x000142094040(lVar6,uStack_19._4_4_);
    }
    cVar4 = func_0x000146682140(_DAT_14e683c78,0x286,1);
    if (cVar4 == '\0') {
      uVar7 = func_0x000142080c60();
      puVar8 = (undefined8 *)func_0x0001414c56e0(uVar7,&lStack_30);
      func_0x00014208c370(*puVar8);
      if (plStack_28 != (longlong *)0x0) {
        LOCK();
        plVar1 = plStack_28 + 1;
        iVar3 = *(int *)plVar1;
        *(int *)plVar1 = *(int *)plVar1 + -1;
        UNLOCK();
        if (iVar3 == 1) {
          (**(code **)*plStack_28)(plStack_28);
          LOCK();
          piVar2 = (int *)((longlong)plStack_28 + 0xc);
          iVar3 = *piVar2;
          *piVar2 = *piVar2 + -1;
          UNLOCK();
          if (iVar3 == 1) {
            (**(code **)(*plStack_28 + 8))(plStack_28);
          }
        }
      }
    }
  }
  return;
}



// FUNCTION 141c28e90 FUN_141c28e90

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c28e90(undefined4 param_1,char param_2)

{
  char cVar1;
  char cVar2;
  longlong lVar3;
  longlong lVar4;
  undefined8 uVar5;
  byte bVar6;
  byte bVar7;
  undefined4 auStackX_10 [2];
  int iStack_28;
  short sStack_24;
  char cStack_22;
  undefined8 uStack_21;
  char cStack_19;
  short sStack_17;
  
  auStackX_10[0] = param_1;
  func_0x0001403a7590(&DAT_14e652c80,auStackX_10);
  if (param_2 != '\0') {
    lVar4 = 0;
    iStack_28 = 0;
    sStack_24 = -1;
    cStack_22 = '\0';
    uStack_21 = 0;
    cStack_19 = '\0';
    sStack_17 = 0;
    func_0x000146ea0be0(&iStack_28,0x13);
    if (_DAT_14e64f730 == 0) {
      lVar3 = func_0x000146e8ba20(0x170);
      if (lVar3 != 0) {
        lVar4 = FUN_141c1fad0(lVar3);
      }
      _DAT_14e64f730 = lVar4;
      (***(code ***)(lVar4 + 0x50))();
    }
    lVar4 = _DAT_14e64f730;
    uVar5 = func_0x00014667eb40(_DAT_14e683c78,0x289);
    lVar3 = func_0x000148aa307c(uVar5,0,0x14dcb4760,0x14dd338b0,0);
    if (lVar4 != 0) {
      func_0x000141c23a90(lVar4);
      cVar1 = cStack_19;
      cVar2 = cStack_22;
      func_0x000141c234b0(lVar4,(int)sStack_17);
      if (iStack_28 != 1) {
        if (iStack_28 == 2) {
          func_0x000141c234a0(lVar4,2,4);
          func_0x000141c234d0(lVar4,(int)sStack_24);
          if (lVar3 == 0) {
            return;
          }
          func_0x000141c1de10(lVar3);
          return;
        }
        if (iStack_28 != 4) {
          return;
        }
      }
      if (cVar2 == '\0') {
        bVar6 = 1;
        bVar7 = -(cVar1 != '\0') & 2;
      }
      else {
        bVar7 = 1;
        bVar6 = -(cVar1 != '\0') & 2;
      }
      func_0x000141c234a0(lVar4,bVar6,bVar7);
      if (lVar3 != 0) {
        cVar2 = func_0x000146682140(_DAT_14e683c78,0x289,1);
        if (cVar2 == '\0') {
          func_0x000146694030(_DAT_14e683c78,0);
          func_0x00014668c520(_DAT_14e683c78,0x289,0,0);
        }
        else {
          func_0x000141c1de10(lVar3);
        }
        func_0x000141c1b0c0(lVar3,uStack_21._4_4_);
      }
    }
  }
  return;
}



// FUNCTION 14209fbe0 FUN_14209fbe0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14209fbe0(longlong param_1,uint param_2)

{
  char cVar1;
  undefined8 *puVar2;
  longlong lVar3;
  undefined8 *puVar4;
  short sVar5;
  undefined8 *puVar6;
  undefined8 *puVar7;
  longlong *plVar8;
  undefined auStack_d8 [32];
  undefined8 uStack_b8;
  undefined auStack_a8 [16];
  int iStack_98;
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
    if (*(longlong *)(param_1 + 0xe0) != 0) {
      func_0x0001420a46f0();
    }
    *(undefined8 *)(param_1 + 0xe0) = 0;
    func_0x00014207fe40(auStack_a8);
    func_0x0001479f8a60(*(undefined8 *)(param_1 + 0xd0),*(undefined2 *)(param_1 + 0x5a),auStack_a8);
    puVar2 = *(undefined8 **)(param_1 + 0xf8);
    cVar1 = *(char *)((longlong)(undefined8 *)puVar2[1] + 0x19);
    puVar4 = puVar2;
    puVar7 = (undefined8 *)puVar2[1];
    while (cVar1 == '\0') {
      if (*(int *)(puVar7 + 4) < iStack_98) {
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
    if (((*(char *)((longlong)puVar4 + 0x19) == '\0') && (*(int *)(puVar4 + 4) <= iStack_98)) &&
       (puVar4 != puVar2)) {
      plVar8 = (longlong *)puVar4[5];
      *(longlong **)(param_1 + 0xe0) = plVar8;
      sVar5 = *(short *)(param_1 + 0x5a);
      if (*(short *)(param_1 + 0x9f) == sVar5) {
        *(int *)(plVar8 + 2) = (int)*(short *)(param_1 + 0xa3);
        *(int *)(*(longlong *)(param_1 + 0xe0) + 0x14) = (int)*(short *)(param_1 + 0xa1);
        lVar3 = *(longlong *)(param_1 + 0xe0);
        *(undefined8 *)(lVar3 + 0xe8) = *(undefined8 *)(param_1 + 0x9f);
        *(undefined2 *)(lVar3 + 0xf0) = *(undefined2 *)(param_1 + 0xa7);
        sVar5 = *(short *)(param_1 + 0x5a);
        plVar8 = *(longlong **)(param_1 + 0xe0);
      }
      if (*(short *)(param_1 + 0xa9) == sVar5) {
        *(int *)(plVar8 + 2) = (int)*(short *)(param_1 + 0xad);
        *(int *)(*(longlong *)(param_1 + 0xe0) + 0x14) = (int)*(short *)(param_1 + 0xab);
        lVar3 = *(longlong *)(param_1 + 0xe0);
        *(undefined8 *)(lVar3 + 0xe8) = *(undefined8 *)(param_1 + 0xa9);
        *(undefined2 *)(lVar3 + 0xf0) = *(undefined2 *)(param_1 + 0xb1);
        sVar5 = *(short *)(param_1 + 0x5a);
        plVar8 = *(longlong **)(param_1 + 0xe0);
      }
      if (*(short *)(param_1 + 0xb3) == sVar5) {
        *(int *)(plVar8 + 2) = (int)*(short *)(param_1 + 0xb7);
        *(int *)(*(longlong *)(param_1 + 0xe0) + 0x14) = (int)*(short *)(param_1 + 0xb5);
        lVar3 = *(longlong *)(param_1 + 0xe0);
        *(undefined8 *)(lVar3 + 0xe8) = *(undefined8 *)(param_1 + 0xb3);
        *(undefined2 *)(lVar3 + 0xf0) = *(undefined2 *)(param_1 + 0xbb);
        sVar5 = *(short *)(param_1 + 0x5a);
        plVar8 = *(longlong **)(param_1 + 0xe0);
      }
      if (*(short *)(param_1 + 0xbd) == sVar5) {
        *(int *)(plVar8 + 2) = (int)*(short *)(param_1 + 0xc1);
        *(int *)(*(longlong *)(param_1 + 0xe0) + 0x14) = (int)*(short *)(param_1 + 0xbf);
        lVar3 = *(longlong *)(param_1 + 0xe0);
        *(undefined8 *)(lVar3 + 0xe8) = *(undefined8 *)(param_1 + 0xbd);
        *(undefined2 *)(lVar3 + 0xf0) = *(undefined2 *)(param_1 + 0xc5);
        plVar8 = *(longlong **)(param_1 + 0xe0);
      }
      (**(code **)(*plVar8 + 0x48))();
    }
    func_0x0001420801b0(auStack_a8);
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_d8);
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
    uVar3 = func_0x000141c0d850(lVar1,0x67);
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


