// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 1424fd160 FUN_1424fd160

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_1424fd160(undefined4 param_1,char param_2)

{
  int iVar1;
  undefined8 uVar2;
  undefined8 uVar3;
  undefined8 uVar4;
  undefined8 uVar5;
  undefined4 auStackX_10 [2];
  int iStack_18;
  undefined4 uStack_14;
  undefined4 uStack_10;
  undefined uStack_c;
  
  auStackX_10[0] = param_1;
  func_0x0001403a7590(&DAT_14e652c80,auStackX_10);
  if (param_2 == '\0') {
    return;
  }
  iStack_18 = 0;
  uStack_14 = 0x6c;
  uStack_10 = 0xffffffff;
  uStack_c = 0;
  func_0x000146ea0be0(&iStack_18,0xd);
  iVar1 = iStack_18;
  if (iStack_18 == 0) {
    return;
  }
  if (iStack_18 == 0xfc) {
    if (_DAT_14e683c78 == 0) goto LAB_1424fd219;
    uVar5 = 0x2d5;
  }
  else {
    if ((iStack_18 != 0x17c) || (_DAT_14e683c78 == 0)) goto LAB_1424fd219;
    uVar5 = 0x605b51b;
  }
  uVar5 = func_0x00014723c170(uVar5);
  func_0x00014668c520(_DAT_14e683c78,0xb3b,uVar5,0);
LAB_1424fd219:
  uVar5 = func_0x00014021a860();
  uVar2 = func_0x000146e8c7d0(&UNK_1499d3210);
  uVar3 = func_0x000146e8c7d0(&UNK_1499d32a0);
  uVar4 = func_0x000146e8c7d0(&UNK_1499d2ff0);
  func_0x000146e938e0(uVar5,0,uVar4,uVar3,0x189,0x14e652c90,uVar2,iVar1);
  return;
}


