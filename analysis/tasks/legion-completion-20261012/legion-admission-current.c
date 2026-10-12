// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141d33e00 FUN_141d33e00

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141d33e00(longlong *param_1)

{
  code *pcVar1;
  undefined auStack_148 [32];
  undefined4 auStack_128 [4];
  undefined auStack_118 [256];
  ulonglong uStack_18;
  
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_148;
  pcVar1 = *(code **)(*param_1 + 0x278);
  func_0x000142ab4ad0(_DAT_14e683c40,auStack_118);
  auStack_128[0] = 0x69;
  func_0x00014250bc60(param_1,auStack_128);
  (*pcVar1)(param_1,8);
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_148);
  return;
}



// FUNCTION 141d33970 FUN_141d33970

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141d33970(longlong *param_1)

{
  undefined auStack_148 [32];
  undefined4 auStack_128 [4];
  undefined auStack_118 [256];
  ulonglong uStack_18;
  
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_148;
  if (*(int *)(param_1[0x10] + 0x18) < 6) {
    func_0x000142ab4ad0(_DAT_14e683c40,auStack_118);
    auStack_128[0] = 0x69;
    func_0x00014250bc60(param_1,auStack_128);
    (**(code **)(*param_1 + 0x290))(param_1,0);
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_148);
  return;
}



// FUNCTION 140696340 FUN_140696340

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_140696340(longlong *param_1)

{
  int iVar1;
  longlong lVar2;
  undefined auStack_148 [32];
  undefined4 auStack_128 [4];
  undefined auStack_118 [256];
  ulonglong uStack_18;
  
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_148;
  lVar2 = func_0x000142ab4ad0(_DAT_14e683c40,auStack_118);
  auStack_128[0] = 0x6b;
  iVar1 = func_0x00014250bc60(param_1,auStack_128);
  if (*(short *)(lVar2 + 0xc + (longlong)iVar1 * 0x18) != 0) {
    (**(code **)(*param_1 + 0x290))(param_1,0);
  }
  (**(code **)(*param_1 + 0x290))(param_1,0);
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_148);
  return;
}



// FUNCTION 14250d570 FUN_14250d570

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14250d570(longlong *param_1)

{
  code *pcVar1;
  undefined auStack_138 [32];
  undefined auStack_118 [256];
  ulonglong uStack_18;
  
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_138;
  if (*(int *)((longlong)param_1 + 0x6c) - 0x65U < 7) {
    pcVar1 = *(code **)(*param_1 + 0x278);
    func_0x000142ab4ad0(_DAT_14e683c40,auStack_118);
    (*pcVar1)(param_1,8);
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_138);
  return;
}



// FUNCTION 14250e2a0 FUN_14250e2a0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

undefined4 FUN_14250e2a0(void)

{
  int iVar1;
  
  iVar1 = func_0x000142ab25e0(_DAT_14e683c40);
  if (iVar1 != 2) {
    iVar1 = func_0x000142ab25e0(_DAT_14e683c40);
    if (iVar1 != 6) {
      return 0;
    }
  }
  return 1;
}


