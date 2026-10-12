// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141c20f60 FUN_141c20f60

longlong FUN_141c20f60(undefined8 param_1,undefined4 param_2)

{
  longlong lVar1;
  
  switch(param_2) {
  case 1:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c259b0(lVar1);
    }
    return lVar1;
  case 2:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c25f90(lVar1);
    }
    return lVar1;
  case 3:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = FUN_141c25ac0(lVar1);
    }
    return lVar1;
  case 4:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = FUN_141c25af0(lVar1);
    }
    return lVar1;
  case 5:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c25d30(lVar1);
    }
    break;
  case 6:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c25bd0(lVar1);
    }
    return lVar1;
  case 7:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c25a90(lVar1);
    }
    return lVar1;
  case 8:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c259e0(lVar1);
    }
    return lVar1;
  case 9:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = FUN_141c25b20(lVar1);
    }
    return lVar1;
  case 10:
    lVar1 = func_0x000146e8ba20(0x168);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c25e90(lVar1);
    }
    return lVar1;
  case 0xb:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c25c80(lVar1);
    }
    return lVar1;
  case 0xc:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c25de0(lVar1);
    }
    return lVar1;
  default:
    return 0;
  case 0xe:
    lVar1 = func_0x000146e8ba20(0x128);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c25f60(lVar1);
    }
    return lVar1;
  case 0xf:
    lVar1 = func_0x000146e8ba20(0x168);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c25fc0(lVar1);
    }
    return lVar1;
  case 0x10:
    lVar1 = func_0x000146e8ba20(0x168);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c26090(lVar1);
    }
    return lVar1;
  case 0x11:
    lVar1 = func_0x000146e8ba20(0x168);
    if (lVar1 != 0) {
      lVar1 = func_0x000141c26160(lVar1);
    }
    return lVar1;
  }
  return lVar1;
}



// FUNCTION 141c25ac0 FUN_141c25ac0

undefined8 * FUN_141c25ac0(undefined8 *param_1)

{
  func_0x000141c26230();
  *param_1 = &UNK_1497ebce8;
  return param_1;
}



// FUNCTION 141c25af0 FUN_141c25af0

undefined8 * FUN_141c25af0(undefined8 *param_1)

{
  func_0x000141c26230();
  *param_1 = &UNK_1497ebd78;
  return param_1;
}



// FUNCTION 141c25b20 FUN_141c25b20

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c25b20(undefined8 *param_1)

{
  undefined auStack_88 [32];
  undefined8 uStack_68;
  undefined8 *puStack_60;
  undefined *puStack_58;
  undefined8 *puStack_50;
  undefined **ppuStack_20;
  ulonglong uStack_18;
  
  uStack_68 = 0xfffffffffffffffe;
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_88;
  puStack_60 = param_1;
  func_0x000141c26230();
  *param_1 = &UNK_1497ec168;
  puStack_58 = &UNK_1497ec570;
  ppuStack_20 = &puStack_58;
  puStack_50 = param_1;
  func_0x000140438ce0(&puStack_58,param_1 + 0x19);
  if (ppuStack_20 != (undefined **)0x0) {
    (**(code **)(*ppuStack_20 + 0x20))
              (ppuStack_20,CONCAT71((int7)((ulonglong)*ppuStack_20 >> 8),ppuStack_20 != &puStack_58)
              );
    ppuStack_20 = (undefined **)0x0;
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_88);
  return;
}


