// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 140698520 FUN_140698520

undefined8 FUN_140698520(longlong param_1)

{
  undefined8 uVar1;
  
  if (*(char *)(param_1 + 0x6a) == -1) {
    return 0xffffffff;
  }
  uVar1 = 5;
  if (1 < (int)*(char *)(param_1 + 0x6a) - 1U) {
    uVar1 = 3;
  }
  return uVar1;
}


