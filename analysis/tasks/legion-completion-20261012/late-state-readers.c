// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 141c3ccc0 FUN_141c3ccc0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_141c3ccc0(void)

{
  undefined8 uVar1;
  undefined *puVar2;
  longlong lVar3;
  longlong lVar4;
  undefined auStack_98 [32];
  undefined2 uStack_78;
  undefined uStack_76;
  undefined4 uStack_75;
  undefined4 uStack_71;
  ulonglong uStack_6d;
  undefined4 uStack_65;
  undefined auStack_61 [48];
  undefined uStack_31;
  uint uStack_2f;
  undefined8 uStack_2b;
  ulonglong uStack_18;
  
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_98;
  uStack_75 = 0xe;
  lVar3 = 4;
  uStack_78 = 0xffff;
  uStack_76 = 0xff;
  puVar2 = auStack_61;
  uStack_71 = 4;
  lVar4 = 4;
  uStack_65 = 0xffffffff;
  uStack_6d = 0xffffffffffffffff;
  do {
    func_0x0001471b0f60(puVar2);
    puVar2 = puVar2 + 0xc;
    lVar4 = lVar4 + -1;
  } while (lVar4 != 0);
  uStack_31 = 0;
  puVar2 = auStack_61;
  uStack_2f = 0;
  uStack_2b = 0;
  do {
    func_0x0001471b3c70(puVar2);
    puVar2 = puVar2 + 0xc;
    lVar3 = lVar3 + -1;
  } while (lVar3 != 0);
  uStack_31 = 0;
  uStack_2f = uStack_2f & 0xffff0000;
  func_0x000146ea0be0(&uStack_78,0x55);
  uVar1 = func_0x000141c38ab0();
  func_0x000141c3a290(uVar1,&uStack_78);
  func_0x000142ac2220(_DAT_14e683c40,uStack_71);
  func_0x000142ac2290(_DAT_14e683c40,uStack_75);
  func_0x000142ac21b0(_DAT_14e683c40,uStack_6d & 0xffffffff);
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_98);
  return;
}



// FUNCTION 1424fddb0 FUN_1424fddb0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_1424fddb0(void)

{
  longlong lVar1;
  undefined *puVar2;
  longlong lVar3;
  undefined auStack_128 [32];
  short asStack_108 [4];
  undefined8 uStack_100;
  longlong lStack_f8;
  undefined2 uStack_e8;
  undefined uStack_e6;
  undefined4 uStack_e5;
  undefined4 uStack_e1;
  ulonglong uStack_dd;
  undefined uStack_d5;
  undefined2 uStack_d3;
  undefined4 uStack_d1;
  undefined auStack_cd [72];
  undefined4 uStack_85;
  undefined4 uStack_81;
  ulonglong uStack_18;
  
  uStack_100 = 0xfffffffffffffffe;
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_128;
  lVar1 = 0;
  asStack_108[0] = 0;
  func_0x000146ea1920(asStack_108);
  if (asStack_108[0] == 0x6b) {
    uStack_e8 = 0xffff;
    uStack_e6 = 0xff;
    uStack_e5 = 0xe;
    uStack_e1 = 4;
    uStack_dd = 0xffffffffffffffff;
    uStack_d1 = 0xffffffff;
    uStack_d5 = 0;
    uStack_d3 = 0;
    puVar2 = auStack_cd;
    lVar3 = 6;
    do {
      func_0x0001471b0f60(puVar2);
      puVar2 = puVar2 + 0xc;
      lVar3 = lVar3 + -1;
    } while (lVar3 != 0);
    uStack_85 = 0xffffffff;
    uStack_81 = 0;
    func_0x000140698280(&uStack_e8);
    func_0x000146ea0be0(&uStack_e8,0xcc);
    if (_DAT_14e638998 == 0) {
      lStack_f8 = func_0x000146e8ba20(0x158);
      if (lStack_f8 != 0) {
        lVar1 = func_0x000140697290(lStack_f8);
      }
      _DAT_14e638998 = lVar1;
      (***(code ***)(lVar1 + 0x50))();
    }
    func_0x00014069a4d0(_DAT_14e638998,&uStack_e8);
    func_0x000142ac2220(_DAT_14e683c40,uStack_e1);
    func_0x000142ac2290(_DAT_14e683c40,uStack_e5);
    func_0x000142ac21b0(_DAT_14e683c40,uStack_dd & 0xffffffff);
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_128);
  return;
}



// FUNCTION 141c390d0 FUN_141c390d0

void FUN_141c390d0(undefined2 *param_1)

{
  undefined4 unaff_EDI;
  
  *(undefined4 *)(param_1 + 8) = unaff_EDI;
  *(undefined8 *)(param_1 + 0xc) = 7;
  *param_1 = (short)unaff_EDI;
  func_0x00014014c8d0(param_1,&DAT_14baf7f08,0);
  return;
}


