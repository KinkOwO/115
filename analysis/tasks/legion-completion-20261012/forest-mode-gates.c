// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 14250e070 FUN_14250e070

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

undefined4 FUN_14250e070(longlong param_1)

{
  int iVar1;
  longlong lVar2;
  
  lVar2 = func_0x000141308bd0(_DAT_14e66c090);
  if ((lVar2 != 0) && (*(longlong *)(param_1 + 0x80) != 0)) {
    iVar1 = func_0x000141c4a150(lVar2);
    if (iVar1 == *(short *)(*(longlong *)(param_1 + 0x80) + 0x2c)) {
      iVar1 = func_0x000146d01ab0(lVar2);
      if (iVar1 == *(short *)(*(longlong *)(param_1 + 0x80) + 0x2e)) {
        return 1;
      }
    }
  }
  return 0;
}



// FUNCTION 14250e170 FUN_14250e170

bool FUN_14250e170(longlong *param_1)

{
  int iVar1;
  
  if (param_1[0x10] == 0) {
    return false;
  }
  iVar1 = (**(code **)(*param_1 + 0x48))();
  return *(int *)(param_1[0x10] + 0x18) == iVar1;
}


