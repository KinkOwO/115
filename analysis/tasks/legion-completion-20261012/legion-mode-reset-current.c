// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 142511820 FUN_142511820

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_142511820(longlong *param_1,int param_2,undefined8 param_3,undefined8 param_4)

{
  int *piVar1;
  int iVar2;
  int iVar3;
  bool bVar4;
  char cVar5;
  longlong *plVar6;
  undefined8 *puVar7;
  uint uVar8;
  uint uVar9;
  undefined auStack_48 [8];
  longlong *plStack_40;
  undefined auStack_38 [8];
  longlong *plStack_30;
  undefined auStack_28 [8];
  longlong *plStack_20;
  
  uVar8 = 0;
  uVar9 = 0;
  iVar3 = *(int *)((longlong)param_1 + 0x74);
  if ((iVar3 == param_2) && (*(char *)((longlong)param_1 + 0x49) == '\0')) {
    return;
  }
  *(int *)((longlong)param_1 + 0x74) = param_2;
  if (*(int *)(param_1 + 0xf) != 2) {
    *(undefined *)(param_1 + 0x11) = 0;
  }
  if (param_2 == 3) {
LAB_1425118c6:
    bVar4 = false;
  }
  else {
    plVar6 = (longlong *)
             func_0x00014667f660(_DAT_14e683c78,auStack_38,param_3,param_4,0,0xfffffffffffffffe);
    uVar8 = 1;
    uVar9 = 1;
    if (*plVar6 == 0) goto LAB_1425118c6;
    puVar7 = (undefined8 *)func_0x00014667f660(_DAT_14e683c78,auStack_48);
    uVar8 = 3;
    uVar9 = 3;
    cVar5 = func_0x000144ad3570(*puVar7);
    if (cVar5 != '\0') goto LAB_1425118c6;
    bVar4 = true;
  }
  if (((uVar8 & 2) != 0) &&
     (uVar8 = uVar8 & 0xfffffffd, uVar9 = uVar8, plStack_40 != (longlong *)0x0)) {
    LOCK();
    plVar6 = plStack_40 + 1;
    iVar2 = *(int *)plVar6;
    *(int *)plVar6 = *(int *)plVar6 + -1;
    UNLOCK();
    if (iVar2 == 1) {
      (**(code **)*plStack_40)(plStack_40);
      LOCK();
      piVar1 = (int *)((longlong)plStack_40 + 0xc);
      iVar2 = *piVar1;
      *piVar1 = *piVar1 + -1;
      UNLOCK();
      uVar9 = uVar8;
      if (iVar2 == 1) {
        (**(code **)(*plStack_40 + 8))(plStack_40);
        uVar9 = uVar8;
      }
    }
  }
  if (((uVar8 & 1) != 0) && (uVar9 = uVar8 & 0xfffffffe, plStack_30 != (longlong *)0x0)) {
    LOCK();
    plVar6 = plStack_30 + 1;
    iVar2 = *(int *)plVar6;
    *(int *)plVar6 = *(int *)plVar6 + -1;
    UNLOCK();
    if (iVar2 == 1) {
      (**(code **)*plStack_30)(plStack_30);
      LOCK();
      piVar1 = (int *)((longlong)plStack_30 + 0xc);
      iVar2 = *piVar1;
      *piVar1 = *piVar1 + -1;
      UNLOCK();
      if (iVar2 == 1) {
        (**(code **)(*plStack_30 + 8))(plStack_30);
      }
    }
  }
  if (bVar4) {
    puVar7 = (undefined8 *)func_0x00014667f660(_DAT_14e683c78,auStack_28);
    func_0x000144ad3550(*puVar7);
    if (plStack_20 != (longlong *)0x0) {
      LOCK();
      plVar6 = plStack_20 + 1;
      iVar2 = *(int *)plVar6;
      *(int *)plVar6 = *(int *)plVar6 + -1;
      UNLOCK();
      if (iVar2 == 1) {
        (**(code **)*plStack_20)(plStack_20);
        LOCK();
        piVar1 = (int *)((longlong)plStack_20 + 0xc);
        iVar2 = *piVar1;
        *piVar1 = *piVar1 + -1;
        UNLOCK();
        if (iVar2 == 1) {
          (**(code **)(*plStack_20 + 8))(plStack_20);
        }
      }
    }
  }
  if ((iVar3 == 1) && (*(int *)((longlong)param_1 + 0x74) == 6)) {
    func_0x0001459b4f40(_DAT_14e66c090,1,1,0,uVar9);
  }
  switch(param_2) {
  case 0:
    (**(code **)(*param_1 + 0x68))(param_1,iVar3);
    *(undefined *)((longlong)param_1 + 0x49) = 0;
    break;
  case 1:
    (**(code **)(*param_1 + 0x70))(param_1,iVar3);
    *(undefined *)((longlong)param_1 + 0x49) = 0;
    goto code_r0x000142511a74;
  case 2:
    (**(code **)(*param_1 + 0x78))(param_1,iVar3);
    *(undefined *)((longlong)param_1 + 0x49) = 0;
    break;
  case 3:
    (**(code **)(*param_1 + 0x80))(param_1,iVar3);
    *(undefined *)((longlong)param_1 + 0x49) = 0;
    break;
  case 4:
    (**(code **)(*param_1 + 0x88))(param_1,iVar3);
    *(undefined *)((longlong)param_1 + 0x49) = 0;
    break;
  case 5:
    (**(code **)(*param_1 + 0x90))(param_1,iVar3);
    *(undefined *)((longlong)param_1 + 0x49) = 0;
    break;
  case 6:
    (**(code **)(*param_1 + 0x98))(param_1,iVar3);
    *(undefined *)((longlong)param_1 + 0x49) = 0;
    break;
  default:
    *(undefined *)((longlong)param_1 + 0x49) = 0;
    if (param_2 != 1) {
      return;
    }
code_r0x000142511a74:
    func_0x000142ab1ab0(_DAT_14e683c40);
  }
  return;
}



// FUNCTION 140696260 FUN_140696260

undefined4 FUN_140696260(longlong param_1)

{
  return *(undefined4 *)(param_1 + 0x78);
}


