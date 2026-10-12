// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 1406a8ce0 FUN_1406a8ce0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

undefined8 FUN_1406a8ce0(longlong *param_1,undefined4 param_2,undefined8 param_3,undefined8 param_4)

{
  char cVar1;
  longlong lVar2;
  uint uVar3;
  longlong **pplVar4;
  undefined8 uVar5;
  
  uVar5 = 0xfffffffffffffffe;
  cVar1 = (**(code **)(*param_1 + 0x298))();
  if (cVar1 == '\0') {
LAB_1406a8dfc:
    uVar5 = 0;
  }
  else {
    if (*(int *)(param_1 + 0x72) != 1) {
      if (*(int *)(param_1 + 0x72) != 2) goto LAB_1406a8dfc;
      cVar1 = func_0x000141bfa430(param_1[0x76]);
      if (cVar1 != '\0') {
        uVar5 = (**(code **)(*(longlong *)param_1[0x76] + 0x2d8))((longlong *)param_1[0x76],param_2)
        ;
        return uVar5;
      }
    }
    uVar3 = 0;
    pplVar4 = (longlong **)(param_1 + 0x74);
    do {
      cVar1 = (**(code **)(**pplVar4 + 0x2d8))(*pplVar4,param_2,param_3,param_4,uVar5);
      if (cVar1 == '\x01') {
        if (_DAT_14e638998 == 0) {
          lVar2 = func_0x000146e8ba20(0x158);
          if (lVar2 != 0) {
            lVar2 = FUN_140697290(lVar2);
          }
          _DAT_14e638998 = lVar2;
          (***(code ***)(lVar2 + 0x50))();
        }
        cVar1 = func_0x00014069a280(_DAT_14e638998);
        if (cVar1 != '\0') {
          *(uint *)((longlong)param_1 + 0x3ec) = uVar3;
          if (*(int *)(param_1 + 0x72) != 2) {
            *(undefined4 *)(param_1 + 0x72) = 2;
            FUN_1406a94a0(param_1,2);
          }
          return 1;
        }
      }
      uVar3 = uVar3 + 1;
      pplVar4 = pplVar4 + 1;
    } while (uVar3 < 2);
    uVar5 = func_0x00014014deb0(param_1,param_2);
  }
  return uVar5;
}



// FUNCTION 1406a94a0 FUN_1406a94a0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_1406a94a0(longlong *param_1,int param_2)

{
  longlong lVar1;
  char cVar2;
  longlong lVar3;
  ulonglong uVar4;
  undefined8 uVar5;
  uint uVar6;
  ulonglong uVar7;
  longlong *plVar8;
  
  if (param_2 == -1) {
    *(undefined4 *)((longlong)param_1 + 0x3ec) = 0xffffffff;
    func_0x00014338f800(param_1[0x71],0);
    func_0x000144a8eab0(param_1[0x76]);
    (**(code **)(*param_1 + 0x10))(param_1,0);
    (**(code **)(*param_1 + 0x18))(param_1,0);
  }
  else if (param_2 == 0) {
    func_0x0001406a8e20();
  }
  else if (param_2 == 1) {
    (**(code **)(*param_1 + 0x10))(param_1,1);
    (**(code **)(*param_1 + 0x18))(param_1,1);
    uVar6 = 0;
    plVar8 = param_1 + 0x74;
    do {
      lVar3 = *plVar8;
      if ((*(int *)(lVar3 + 0x38c) - 2U < 2) && (*(int *)(lVar3 + 0x38c) != 1)) {
        func_0x0001406a8fd0(lVar3,1);
        *(undefined4 *)(lVar3 + 0x38c) = 1;
      }
      uVar6 = uVar6 + 1;
      plVar8 = plVar8 + 1;
    } while (uVar6 < 2);
    if (param_1[0x73] != 0) {
      func_0x0001406b0ad0(param_1[0x73],param_1,*(undefined4 *)((longlong)param_1 + 0x3ec));
    }
  }
  else if (param_2 == 2) {
    (**(code **)(*param_1 + 0x10))(param_1,1);
    (**(code **)(*param_1 + 0x18))(param_1,1);
    uVar7 = 0;
    if (_DAT_14e638998 == 0) {
      lVar3 = func_0x000146e8ba20(0x158);
      uVar4 = uVar7;
      if (lVar3 != 0) {
        uVar4 = FUN_140697290(lVar3);
      }
      _DAT_14e638998 = uVar4;
      (***(code ***)(uVar4 + 0x50))();
    }
    cVar2 = func_0x00014069a280(_DAT_14e638998);
    if (cVar2 != '\0') {
      func_0x0001406a9b40(param_1,*(undefined4 *)((longlong)param_1 + 0x3ec));
      func_0x00014069e410(*(undefined4 *)((longlong)param_1 + 0x3ec));
    }
    plVar8 = param_1 + 0x74;
    lVar3 = 2;
    do {
      lVar1 = *plVar8;
      if (*(int *)((longlong)param_1 + 0x3ec) == (int)uVar7) {
        if (*(int *)(lVar1 + 0x38c) != 2) {
          func_0x0001406a8fd0(lVar1,2);
          *(undefined4 *)(lVar1 + 0x38c) = 2;
        }
      }
      else if (*(int *)(lVar1 + 0x38c) != 3) {
        func_0x0001406a8fd0(lVar1,3);
        *(undefined4 *)(lVar1 + 0x38c) = 3;
      }
      uVar7 = (ulonglong)((int)uVar7 + 1);
      plVar8 = plVar8 + 1;
      lVar3 = lVar3 + -1;
    } while (lVar3 != 0);
    if (param_1[0x73] != 0) {
      func_0x0001406b0ad0(param_1[0x73],param_1[0x76],1);
    }
    uVar5 = func_0x000146e8c7d0(&UNK_1492e8820);
    func_0x000145a31380(uVar5,0xffffffff,0,0,0xffffffff,0xffffffff,0);
  }
  return;
}



// FUNCTION 140697290 FUN_140697290

undefined8 * FUN_140697290(undefined8 *param_1)

{
  longlong *plVar1;
  longlong lVar2;
  undefined8 uVar3;
  longlong lVar4;
  
  func_0x000146ebb4e0();
  *param_1 = &UNK_1491ef558;
  uVar3 = 0;
  param_1[8] = 0;
  param_1[9] = 0;
  lVar2 = func_0x000146e8ba20(0x48);
  *(longlong *)lVar2 = lVar2;
  *(longlong *)(lVar2 + 8) = lVar2;
  *(longlong *)(lVar2 + 0x10) = lVar2;
  *(undefined2 *)(lVar2 + 0x18) = 0x101;
  param_1[8] = lVar2;
  *param_1 = &UNK_1492e1950;
  param_1[10] = &UNK_1492e1bb8;
  *(undefined2 *)(param_1 + 0xd) = 0xffff;
  *(undefined *)((longlong)param_1 + 0x6a) = 0xff;
  *(undefined4 *)((longlong)param_1 + 0x6b) = 0xe;
  *(undefined4 *)((longlong)param_1 + 0x6f) = 4;
  *(undefined8 *)((longlong)param_1 + 0x73) = 0xffffffffffffffff;
  *(undefined4 *)((longlong)param_1 + 0x7f) = 0xffffffff;
  *(undefined *)((longlong)param_1 + 0x7b) = 0;
  *(undefined2 *)((longlong)param_1 + 0x7d) = 0;
  lVar2 = (longlong)param_1 + 0x83;
  lVar4 = 6;
  do {
    func_0x0001471b0f60(lVar2);
    lVar2 = lVar2 + 0xc;
    lVar4 = lVar4 + -1;
  } while (lVar4 != 0);
  *(undefined4 *)((longlong)param_1 + 0xcb) = 0xffffffff;
  *(undefined4 *)((longlong)param_1 + 0xcf) = 0;
  func_0x000140698280(param_1 + 0xd);
  plVar1 = param_1 + 0x27;
  *plVar1 = 0;
  param_1[0x28] = 0;
  lVar2 = func_0x000146e8ba20(0x30);
  *(longlong *)lVar2 = lVar2;
  *(longlong *)(lVar2 + 8) = lVar2;
  *(longlong *)(lVar2 + 0x10) = lVar2;
  *(undefined2 *)(lVar2 + 0x18) = 0x101;
  *plVar1 = lVar2;
  *(undefined4 *)((longlong)param_1 + 0x154) = 0;
  param_1[0xb] = 0;
  param_1[0xc] = 0;
  lVar2 = *plVar1;
  func_0x00014014ebd0(plVar1,plVar1,*(undefined8 *)(lVar2 + 8));
  *(longlong *)(lVar2 + 8) = lVar2;
  *(longlong *)lVar2 = lVar2;
  *(longlong *)(lVar2 + 0x10) = lVar2;
  param_1[0x28] = 0;
  *(undefined2 *)(param_1 + 0x29) = 0;
  *(undefined4 *)((longlong)param_1 + 0x14c) = 0xffffffff;
  *(undefined *)(param_1 + 0x2a) = 0;
  lVar2 = func_0x000146e8ba20(0x188);
  if (lVar2 != 0) {
    uVar3 = FUN_140695cd0(lVar2,0x6b);
  }
  param_1[0xb] = uVar3;
  return param_1;
}


