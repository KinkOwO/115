// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 14069a360 FUN_14069a360

/* WARNING: Removing unreachable block (ram,0x0001406afaa3) */
/* WARNING: Removing unreachable block (ram,0x0001406afa9a) */
/* WARNING: Removing unreachable block (ram,0x0001406afb8d) */
/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14069a360(longlong param_1,int *param_2)

{
  undefined4 uVar1;
  char cVar2;
  int iVar3;
  undefined8 uVar4;
  longlong lVar5;
  longlong *plVar6;
  undefined8 unaff_RBX;
  
  if (*param_2 == 1) {
    if (*(char *)((longlong)param_2 + 5) == '\0') {
      uVar1 = *(undefined4 *)((longlong)param_2 + 6);
      if (*(char *)(param_1 + 0x149) == '\0') {
        *(undefined *)(param_1 + 0x149) = 1;
      }
      *(undefined *)(param_1 + 0x150) = 1;
      uVar4 = func_0x00014667eb40(_DAT_14e683c78,0xf5d);
      lVar5 = func_0x000148aa307c(uVar4,0,0x14dcb4760,0x14dcba8c8,0);
      if (lVar5 == 0) {
        return;
      }
      func_0x0001406afc60(lVar5,uVar1);
      return;
    }
    *(undefined *)(param_1 + 0x150) = 0;
    uVar4 = func_0x00014667eb40(_DAT_14e683c78,0xf5d);
    lVar5 = func_0x000148aa307c(uVar4,0,0x14dcb4760,0x14dcba8c8,0);
    if (lVar5 != 0) {
      func_0x0001406afca0(lVar5);
    }
  }
  else {
    if (*param_2 != 2) {
      return;
    }
    *(undefined *)(param_1 + 0x150) = 0;
    if ((*(longlong **)(param_1 + 0x58) != (longlong *)0x0) &&
       (iVar3 = (**(code **)(**(longlong **)(param_1 + 0x58) + 0x28))(), iVar3 == 2)) {
      cVar2 = (**(code **)(**(longlong **)(param_1 + 0x58) + 0x1c0))();
      if ((cVar2 == '\0') || (cVar2 = func_0x000145f146c0(), cVar2 == '\0')) {
        if (_DAT_14e683c78 != 0) {
          uVar4 = func_0x00014723c170(0x5f7390d);
          func_0x00014668c520(_DAT_14e683c78,0xb3b,uVar4,0);
        }
      }
      else {
        func_0x0001424fe290(0x6b,*(undefined4 *)(param_1 + 0x73));
      }
    }
  }
  uVar4 = func_0x00014667eb40(_DAT_14e683c78);
  lVar5 = func_0x000148aa307c(uVar4,0,0x14dcb4760,0x14dcba8c8,0);
  if ((lVar5 == 0) || (cVar2 = func_0x0001406af170(lVar5), cVar2 != '\0')) {
    return;
  }
  if ((*(int *)(lVar5 + 0x5e8) != 0) && (cVar2 = func_0x0001406af0d0(), cVar2 != '\0')) {
    *(undefined4 *)(lVar5 + 0x5e8) = 0;
    (**(code **)(**(longlong **)(lVar5 + 0x5f0) + 0x10))(*(longlong **)(lVar5 + 0x5f0),0);
    func_0x000146ee0490(*(undefined8 *)(lVar5 + 0x600),6,7,8,9,0xffffffff,unaff_RBX);
    func_0x0001406a35f0(*(undefined8 *)(lVar5 + 0x680));
    func_0x0001406b0a70();
    if ((*(uint *)(lVar5 + 0x2e8) < 0x16) &&
       ((*(longlong *)(lVar5 + 0x188 + (longlong)(int)*(uint *)(lVar5 + 0x2e8) * 0x10) != 0 &&
        (cVar2 = func_0x000146ecff30(), cVar2 != '\0')))) {
      func_0x000146ed37a0(*(undefined8 *)(lVar5 + 0x188 + (longlong)*(int *)(lVar5 + 0x2e8) * 0x10),
                          0xffffffff);
    }
    func_0x0001467a8b30(lVar5);
    func_0x0001467a8a70(lVar5);
    uVar4 = func_0x000146e8c7d0(&UNK_1492ea340);
    lVar5 = func_0x000145a0f110(uVar4,0x138);
    if (lVar5 == 0) {
      return;
    }
    uVar4 = func_0x000146e8c7d0(&UNK_1492ea340);
    plVar6 = (longlong *)func_0x000145a0f110(uVar4,0x13a);
                    /* WARNING: Could not recover jumptable at 0x0001406afc51. Too many branches */
                    /* WARNING: Treating indirect jump as call */
    (**(code **)(*plVar6 + 0x88))(plVar6,0,1);
    return;
  }
  return;
}


