// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141c135c0 FUN_141c135c0

void FUN_141c135c0(longlong param_1)

{
  undefined8 uVar1;
  longlong lVar2;
  undefined8 uVar3;
  undefined auStack_20 [24];
  
  uVar3 = 0xfffffffffffffffe;
  uVar1 = *(undefined8 *)(param_1 + 0x9e0);
  lVar2 = func_0x000146e8ba20(0x28);
  if (lVar2 != 0) {
    auStack_20._8_8_ = *(longlong *)(param_1 + 0xc70);
    if (auStack_20._8_8_ != 0) {
      LOCK();
      *(int *)(auStack_20._8_8_ + 8) = *(int *)(auStack_20._8_8_ + 8) + 1;
      UNLOCK();
      auStack_20._8_8_ = *(longlong *)(param_1 + 0xc70);
    }
    auStack_20._0_8_ = *(undefined8 *)(param_1 + 0xc68);
    lVar2 = func_0x00014338bef0(lVar2,auStack_20,0,1,uVar3);
  }
  func_0x00014338f710(uVar1,lVar2,1);
  return;
}



// FUNCTION 14667eb40 FUN_14667eb40

longlong * FUN_14667eb40(longlong param_1,int param_2)

{
  longlong lVar1;
  char cVar2;
  int iVar3;
  longlong *plVar4;
  longlong **pplVar5;
  longlong *plStackX_8;
  
  lVar1 = param_1 + (longlong)param_2 * 0x18;
  if ((((longlong **)(lVar1 + 0x1d8) == (longlong **)0x0) ||
      (pplVar5 = *(longlong ***)(longlong **)(lVar1 + 0x1d8),
      pplVar5 == *(longlong ***)(lVar1 + 0x1e0))) || (plVar4 = *pplVar5, plVar4 == (longlong *)0x0))
  {
    if (*(longlong ***)(lVar1 + 0x18958) == *(longlong ***)(lVar1 + 0x18960)) {
      pplVar5 = *(longlong ***)(param_1 + 0x310d8);
      if (pplVar5 != *(longlong ***)(param_1 + 0x310e0)) {
        do {
          plVar4 = *pplVar5;
          iVar3 = (**(code **)(*plVar4 + 0x128))(plVar4);
          if (iVar3 == param_2) {
            return plVar4;
          }
          pplVar5 = pplVar5 + 1;
        } while (pplVar5 != *(longlong ***)(param_1 + 0x310e0));
      }
      plVar4 = (longlong *)func_0x000146753320(param_2,param_1);
      plStackX_8 = plVar4;
      cVar2 = func_0x000146682350(param_2);
      if (cVar2 == '\0') {
        pplVar5 = *(longlong ***)(lVar1 + 0x18960);
        if (pplVar5 == *(longlong ***)(lVar1 + 0x18968)) {
          func_0x0001401c06f0(lVar1 + 0x18958,pplVar5,&plStackX_8);
          plVar4 = plStackX_8;
        }
        else {
          *pplVar5 = plVar4;
          *(longlong *)(lVar1 + 0x18960) = *(longlong *)(lVar1 + 0x18960) + 8;
        }
      }
      if (plVar4 != (longlong *)0x0) {
        (**(code **)(*plVar4 + 0x2a8))(plVar4,plVar4);
      }
    }
    else {
      plVar4 = **(longlong ***)(lVar1 + 0x18958);
    }
  }
  return plVar4;
}



// FUNCTION 141c1a3b0 FUN_141c1a3b0

void FUN_141c1a3b0(longlong **param_1,undefined param_2)

{
  longlong **pplVar1;
  longlong lVar2;
  
  (**(code **)(**param_1 + 0x10))();
  pplVar1 = param_1 + 6;
  lVar2 = 3;
  do {
    (**(code **)(*pplVar1[-4] + 0x10))(pplVar1[-4],param_2);
    (**(code **)(*pplVar1[-2] + 0x10))(pplVar1[-2],param_2);
    (**(code **)(**pplVar1 + 0x10))(*pplVar1,param_2);
    (**(code **)(*pplVar1[2] + 0x10))(pplVar1[2],param_2);
    pplVar1 = pplVar1 + 0xf;
    lVar2 = lVar2 + -1;
  } while (lVar2 != 0);
  (**(code **)(*param_1[0x43] + 0x10))(param_1[0x43],param_2);
  (**(code **)(*param_1[0x2f] + 0x10))(param_1[0x2f],param_2);
  (**(code **)(*param_1[0x31] + 0x10))(param_1[0x31],param_2);
  (**(code **)(*param_1[0x35] + 0x10))(param_1[0x35],param_2);
  (**(code **)(*param_1[0x37] + 0x10))(param_1[0x37],param_2);
  (**(code **)(*param_1[0x39] + 0x10))(param_1[0x39],param_2);
  (**(code **)(*param_1[0x3b] + 0x10))(param_1[0x3b],param_2);
  (**(code **)(*param_1[0x3f] + 0x10))(param_1[0x3f],param_2);
  (**(code **)(*param_1[0x47] + 0x10))(param_1[0x47],param_2);
  (**(code **)(*param_1[0x49] + 0x10))(param_1[0x49],param_2);
  (**(code **)(*param_1[0x4b] + 0x10))(param_1[0x4b],param_2);
  (**(code **)(*param_1[0x4d] + 0x10))(param_1[0x4d],param_2);
  (**(code **)(*param_1[0x4f] + 0x10))(param_1[0x4f],param_2);
  (**(code **)(*param_1[0x33] + 0x10))(param_1[0x33],param_2);
                    /* WARNING: Could not recover jumptable at 0x000141c1a532. Too many branches */
                    /* WARNING: Treating indirect jump as call */
  (**(code **)(*param_1[0x3d] + 0x10))(param_1[0x3d],param_2);
  return;
}


