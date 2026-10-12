// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 1420a07e0 FUN_1420a07e0

void FUN_1420a07e0(longlong param_1)

{
  int iVar1;
  code *pcVar2;
  longlong *plVar3;
  undefined8 uVar4;
  undefined8 uVar5;
  undefined8 uVar6;
  undefined *puVar7;
  ulonglong uVar8;
  longlong lVar9;
  longlong lVar10;
  undefined4 uVar11;
  undefined auStack_d0 [16];
  longlong lStack_c0;
  undefined auStack_b8 [24];
  undefined auStack_a0 [24];
  undefined auStack_88 [24];
  undefined auStack_70 [24];
  undefined auStack_58 [32];
  undefined8 uStack_38;
  
  uStack_38 = 0xfffffffffffffffe;
  if (((*(int *)(param_1 + 99) != 6) && (*(int *)(param_1 + 99) != 1)) ||
     (*(int *)(param_1 + 0x6b) == -1)) {
LAB_1420a0e4c:
    if ((*(int *)(param_1 + 99) - 3U & 0xfffffffd) != 0) {
      return;
    }
    plVar3 = (longlong *)func_0x00014209cc50(*(undefined8 *)(param_1 + 0xd0),auStack_b8);
    uVar4 = func_0x00014209adc0(auStack_70,*plVar3 + 0x108);
    plVar3 = (longlong *)func_0x00014209cdb0(*(undefined8 *)(param_1 + 0xd0),auStack_a0);
    uVar5 = func_0x000140166530(auStack_88,*plVar3 + 0x108);
    func_0x000140166530(auStack_d0,*(longlong *)(param_1 + 0xd0) + 0x2f8);
    uVar6 = func_0x00014014c810(auStack_58,auStack_d0._0_8_ + 0x160);
    uVar11 = func_0x00014209c4f0(param_1,uVar6,uVar5,uVar4,0);
    if (auStack_d0._0_8_ == 0) goto LAB_1420a0f7c;
    lVar10 = auStack_d0._8_8_;
    lVar9 = auStack_d0._0_8_;
    if (auStack_d0._0_8_ != auStack_d0._8_8_) {
      do {
        uVar11 = func_0x00014014c710(lVar9);
        lVar9 = lVar9 + 0x20;
      } while (lVar9 != lVar10);
    }
    uVar8 = lStack_c0 - auStack_d0._0_8_ & 0xffffffffffffffe0;
    lVar10 = auStack_d0._0_8_;
    if (0xfff < uVar8) {
      lVar10 = *(longlong *)(auStack_d0._0_8_ + -8);
      if (0x1f < (auStack_d0._0_8_ - lVar10) - 8U) {
        func_0x000148aaf304(uVar11,uVar8 + 0x27);
        goto LAB_1420a0fac;
      }
    }
    func_0x000146e9f3a0(lVar10);
    auStack_d0 = ZEXT816(0);
    lStack_c0 = 0;
LAB_1420a0f7c:
    func_0x000141ed24a0(auStack_a0);
    func_0x00014209c320(auStack_b8);
    return;
  }
  iVar1 = *(int *)(param_1 + 0x77 + (longlong)*(int *)(param_1 + 0x6b) * 8);
  if (iVar1 == 0) {
    plVar3 = (longlong *)func_0x00014209cc50(*(undefined8 *)(param_1 + 0xd0),auStack_70);
    uVar4 = func_0x00014209adc0(auStack_b8,*plVar3 + 0x78);
    plVar3 = (longlong *)func_0x00014209cdb0(*(undefined8 *)(param_1 + 0xd0),auStack_88);
    uVar5 = func_0x000140166530(auStack_a0,*plVar3 + 0x78);
    func_0x000140166530(auStack_d0,*(longlong *)(param_1 + 0xd0) + 0x2f8);
    uVar6 = func_0x00014014c810(auStack_58,auStack_d0._0_8_ + 0xa0);
    func_0x00014209c4f0(param_1,uVar6,uVar5,uVar4,0);
    if (auStack_d0._0_8_ == 0) {
LAB_1420a095a:
      func_0x000141ed24a0(auStack_88);
      puVar7 = auStack_70;
LAB_1420a0e47:
      func_0x00014209c320(puVar7);
      goto LAB_1420a0e4c;
    }
    lVar10 = auStack_d0._8_8_;
    lVar9 = auStack_d0._0_8_;
    if (auStack_d0._0_8_ != auStack_d0._8_8_) {
      do {
        func_0x00014014c710(lVar9);
        lVar9 = lVar9 + 0x20;
      } while (lVar9 != lVar10);
    }
    uVar8 = lStack_c0 - auStack_d0._0_8_ & 0xffffffffffffffe0;
    lVar10 = auStack_d0._0_8_;
    if (uVar8 < 0x1000) {
LAB_1420a0946:
      func_0x000146e9f3a0(lVar10,uVar8);
      auStack_d0 = ZEXT816(0);
      lStack_c0 = 0;
      goto LAB_1420a095a;
    }
    uVar8 = uVar8 + 0x27;
    lVar10 = *(longlong *)(auStack_d0._0_8_ + -8);
    if ((auStack_d0._0_8_ - lVar10) - 8U < 0x20) goto LAB_1420a0946;
LAB_1420a0fac:
    func_0x000148aaf304();
LAB_1420a0fb2:
    func_0x000148aaf304();
LAB_1420a0fb8:
    func_0x000148aaf304();
  }
  else {
    if (iVar1 == 1) {
      plVar3 = (longlong *)func_0x00014209cc50(*(undefined8 *)(param_1 + 0xd0),auStack_b8);
      uVar4 = func_0x00014209adc0(auStack_70,*plVar3 + 0x60);
      plVar3 = (longlong *)func_0x00014209cdb0(*(undefined8 *)(param_1 + 0xd0),auStack_a0);
      uVar5 = func_0x000140166530(auStack_88,*plVar3 + 0x60);
      func_0x000140166530(auStack_d0,*(longlong *)(param_1 + 0xd0) + 0x2f8);
      uVar6 = func_0x00014014c810(auStack_58,auStack_d0._0_8_ + 0x80);
      func_0x00014209c4f0(param_1,uVar6,uVar5,uVar4,0);
      if (auStack_d0._0_8_ == 0) {
LAB_1420a0a99:
        func_0x000141ed24a0(auStack_a0);
LAB_1420a0e43:
        puVar7 = auStack_b8;
        goto LAB_1420a0e47;
      }
      lVar10 = auStack_d0._8_8_;
      lVar9 = auStack_d0._0_8_;
      if (auStack_d0._0_8_ != auStack_d0._8_8_) {
        do {
          func_0x00014014c710(lVar9);
          lVar9 = lVar9 + 0x20;
        } while (lVar9 != lVar10);
      }
      uVar8 = lStack_c0 - auStack_d0._0_8_ & 0xffffffffffffffe0;
      lVar10 = auStack_d0._0_8_;
      if (uVar8 < 0x1000) {
LAB_1420a0a85:
        func_0x000146e9f3a0(lVar10,uVar8);
        auStack_d0 = ZEXT816(0);
        lStack_c0 = 0;
        goto LAB_1420a0a99;
      }
      uVar8 = uVar8 + 0x27;
      lVar10 = *(longlong *)(auStack_d0._0_8_ + -8);
      if ((auStack_d0._0_8_ - lVar10) - 8U < 0x20) goto LAB_1420a0a85;
      goto LAB_1420a0fb2;
    }
    if (iVar1 == 2) {
      plVar3 = (longlong *)func_0x00014209cc50(*(undefined8 *)(param_1 + 0xd0),auStack_b8);
      uVar4 = func_0x00014209adc0(auStack_70,*plVar3 + 0x48);
      plVar3 = (longlong *)func_0x00014209cdb0(*(undefined8 *)(param_1 + 0xd0),auStack_a0);
      uVar5 = func_0x000140166530(auStack_88,*plVar3 + 0x48);
      func_0x000140166530(auStack_d0,*(longlong *)(param_1 + 0xd0) + 0x2f8);
      uVar6 = func_0x00014014c810(auStack_58,auStack_d0._0_8_ + 0x60);
      func_0x00014209c4f0(param_1,uVar6,uVar5,uVar4,0);
      if (auStack_d0._0_8_ == 0) {
LAB_1420a0bcd:
        func_0x000141ed24a0(auStack_a0);
        goto LAB_1420a0e43;
      }
      lVar10 = auStack_d0._8_8_;
      lVar9 = auStack_d0._0_8_;
      if (auStack_d0._0_8_ != auStack_d0._8_8_) {
        do {
          func_0x00014014c710(lVar9);
          lVar9 = lVar9 + 0x20;
        } while (lVar9 != lVar10);
      }
      uVar8 = lStack_c0 - auStack_d0._0_8_ & 0xffffffffffffffe0;
      lVar10 = auStack_d0._0_8_;
      if (uVar8 < 0x1000) {
LAB_1420a0bb9:
        func_0x000146e9f3a0(lVar10,uVar8);
        auStack_d0 = ZEXT816(0);
        lStack_c0 = 0;
        goto LAB_1420a0bcd;
      }
      uVar8 = uVar8 + 0x27;
      lVar10 = *(longlong *)(auStack_d0._0_8_ + -8);
      if ((auStack_d0._0_8_ - lVar10) - 8U < 0x20) goto LAB_1420a0bb9;
      goto LAB_1420a0fb8;
    }
    if (iVar1 != 3) {
      if (iVar1 != 4) goto LAB_1420a0e4c;
      plVar3 = (longlong *)func_0x00014209cc50(*(undefined8 *)(param_1 + 0xd0),auStack_b8);
      uVar4 = func_0x00014209adc0(auStack_70,*plVar3 + 0x18);
      plVar3 = (longlong *)func_0x00014209cdb0(*(undefined8 *)(param_1 + 0xd0),auStack_a0);
      uVar5 = func_0x000140166530(auStack_88,*plVar3 + 0x18);
      func_0x000140166530(auStack_d0,*(longlong *)(param_1 + 0xd0) + 0x2f8);
      uVar6 = func_0x00014014c810(auStack_58,auStack_d0._0_8_ + 0x20);
      func_0x00014209c4f0(param_1,uVar6,uVar5,uVar4,0);
      if (auStack_d0._0_8_ != 0) {
        lVar10 = auStack_d0._8_8_;
        lVar9 = auStack_d0._0_8_;
        if (auStack_d0._0_8_ != auStack_d0._8_8_) {
          do {
            func_0x00014014c710(lVar9);
            lVar9 = lVar9 + 0x20;
          } while (lVar9 != lVar10);
        }
        uVar8 = lStack_c0 - auStack_d0._0_8_ & 0xffffffffffffffe0;
        lVar10 = auStack_d0._0_8_;
        if (0xfff < uVar8) {
          uVar8 = uVar8 + 0x27;
          lVar10 = *(longlong *)(auStack_d0._0_8_ + -8);
          if (0x1f < (auStack_d0._0_8_ - lVar10) - 8U) goto LAB_1420a0fc4;
        }
        func_0x000146e9f3a0(lVar10,uVar8);
        auStack_d0 = ZEXT816(0);
        lStack_c0 = 0;
      }
      func_0x000141ed24a0(auStack_a0);
      goto LAB_1420a0e43;
    }
    plVar3 = (longlong *)func_0x00014209cc50(*(undefined8 *)(param_1 + 0xd0),auStack_b8);
    uVar4 = func_0x00014209adc0(auStack_70,*plVar3 + 0x30);
    plVar3 = (longlong *)func_0x00014209cdb0(*(undefined8 *)(param_1 + 0xd0),auStack_a0);
    uVar5 = func_0x000140166530(auStack_88,*plVar3 + 0x30);
    func_0x000140166530(auStack_d0,*(longlong *)(param_1 + 0xd0) + 0x2f8);
    uVar6 = func_0x00014014c810(auStack_58,auStack_d0._0_8_ + 0x40);
    func_0x00014209c4f0(param_1,uVar6,uVar5,uVar4,0);
    if (auStack_d0._0_8_ == 0) {
LAB_1420a0d01:
      func_0x000141ed24a0(auStack_a0);
      goto LAB_1420a0e43;
    }
    lVar10 = auStack_d0._8_8_;
    lVar9 = auStack_d0._0_8_;
    if (auStack_d0._0_8_ != auStack_d0._8_8_) {
      do {
        func_0x00014014c710(lVar9);
        lVar9 = lVar9 + 0x20;
      } while (lVar9 != lVar10);
    }
    uVar8 = lStack_c0 - auStack_d0._0_8_ & 0xffffffffffffffe0;
    lVar10 = auStack_d0._0_8_;
    if (uVar8 < 0x1000) {
LAB_1420a0ced:
      func_0x000146e9f3a0(lVar10,uVar8);
      auStack_d0 = ZEXT816(0);
      lStack_c0 = 0;
      goto LAB_1420a0d01;
    }
    uVar8 = uVar8 + 0x27;
    lVar10 = *(longlong *)(auStack_d0._0_8_ + -8);
    if ((auStack_d0._0_8_ - lVar10) - 8U < 0x20) goto LAB_1420a0ced;
  }
  func_0x000148aaf304();
LAB_1420a0fc4:
  func_0x000148aaf304();
  pcVar2 = (code *)swi(3);
  (*pcVar2)();
  return;
}



// FUNCTION 14209fbe0 FUN_14209fbe0

/* WARNING: Globals starting with '_' overlap smaller symbols at the same address */

void FUN_14209fbe0(longlong param_1,uint param_2)

{
  char cVar1;
  undefined8 *puVar2;
  longlong lVar3;
  undefined8 *puVar4;
  short sVar5;
  undefined8 *puVar6;
  undefined8 *puVar7;
  longlong *plVar8;
  undefined auStack_d8 [32];
  undefined8 uStack_b8;
  undefined auStack_a8 [16];
  int iStack_98;
  ulonglong uStack_18;
  
  uStack_b8 = 0xfffffffffffffffe;
  uStack_18 = _DAT_14dca9408 ^ (ulonglong)auStack_d8;
  if (param_2 == 0xffffffff) {
    param_2 = (uint)*(ushort *)(param_1 + 0x5a);
  }
  else {
    *(short *)(param_1 + 0x5a) = (short)param_2;
  }
  if ((short)param_2 != -1) {
    if (*(longlong *)(param_1 + 0xe0) != 0) {
      func_0x0001420a46f0();
    }
    *(undefined8 *)(param_1 + 0xe0) = 0;
    func_0x00014207fe40(auStack_a8);
    func_0x0001479f8a60(*(undefined8 *)(param_1 + 0xd0),*(undefined2 *)(param_1 + 0x5a),auStack_a8);
    puVar2 = *(undefined8 **)(param_1 + 0xf8);
    cVar1 = *(char *)((longlong)(undefined8 *)puVar2[1] + 0x19);
    puVar4 = puVar2;
    puVar7 = (undefined8 *)puVar2[1];
    while (cVar1 == '\0') {
      if (*(int *)(puVar7 + 4) < iStack_98) {
        puVar6 = (undefined8 *)puVar7[2];
        puVar7 = puVar4;
      }
      else {
        puVar6 = (undefined8 *)*puVar7;
      }
      puVar4 = puVar7;
      puVar7 = puVar6;
      cVar1 = *(char *)((longlong)puVar6 + 0x19);
    }
    if (((*(char *)((longlong)puVar4 + 0x19) == '\0') && (*(int *)(puVar4 + 4) <= iStack_98)) &&
       (puVar4 != puVar2)) {
      plVar8 = (longlong *)puVar4[5];
      *(longlong **)(param_1 + 0xe0) = plVar8;
      sVar5 = *(short *)(param_1 + 0x5a);
      if (*(short *)(param_1 + 0x9f) == sVar5) {
        *(int *)(plVar8 + 2) = (int)*(short *)(param_1 + 0xa3);
        *(int *)(*(longlong *)(param_1 + 0xe0) + 0x14) = (int)*(short *)(param_1 + 0xa1);
        lVar3 = *(longlong *)(param_1 + 0xe0);
        *(undefined8 *)(lVar3 + 0xe8) = *(undefined8 *)(param_1 + 0x9f);
        *(undefined2 *)(lVar3 + 0xf0) = *(undefined2 *)(param_1 + 0xa7);
        sVar5 = *(short *)(param_1 + 0x5a);
        plVar8 = *(longlong **)(param_1 + 0xe0);
      }
      if (*(short *)(param_1 + 0xa9) == sVar5) {
        *(int *)(plVar8 + 2) = (int)*(short *)(param_1 + 0xad);
        *(int *)(*(longlong *)(param_1 + 0xe0) + 0x14) = (int)*(short *)(param_1 + 0xab);
        lVar3 = *(longlong *)(param_1 + 0xe0);
        *(undefined8 *)(lVar3 + 0xe8) = *(undefined8 *)(param_1 + 0xa9);
        *(undefined2 *)(lVar3 + 0xf0) = *(undefined2 *)(param_1 + 0xb1);
        sVar5 = *(short *)(param_1 + 0x5a);
        plVar8 = *(longlong **)(param_1 + 0xe0);
      }
      if (*(short *)(param_1 + 0xb3) == sVar5) {
        *(int *)(plVar8 + 2) = (int)*(short *)(param_1 + 0xb7);
        *(int *)(*(longlong *)(param_1 + 0xe0) + 0x14) = (int)*(short *)(param_1 + 0xb5);
        lVar3 = *(longlong *)(param_1 + 0xe0);
        *(undefined8 *)(lVar3 + 0xe8) = *(undefined8 *)(param_1 + 0xb3);
        *(undefined2 *)(lVar3 + 0xf0) = *(undefined2 *)(param_1 + 0xbb);
        sVar5 = *(short *)(param_1 + 0x5a);
        plVar8 = *(longlong **)(param_1 + 0xe0);
      }
      if (*(short *)(param_1 + 0xbd) == sVar5) {
        *(int *)(plVar8 + 2) = (int)*(short *)(param_1 + 0xc1);
        *(int *)(*(longlong *)(param_1 + 0xe0) + 0x14) = (int)*(short *)(param_1 + 0xbf);
        lVar3 = *(longlong *)(param_1 + 0xe0);
        *(undefined8 *)(lVar3 + 0xe8) = *(undefined8 *)(param_1 + 0xbd);
        *(undefined2 *)(lVar3 + 0xf0) = *(undefined2 *)(param_1 + 0xc5);
        plVar8 = *(longlong **)(param_1 + 0xe0);
      }
      (**(code **)(*plVar8 + 0x48))();
    }
    func_0x0001420801b0(auStack_a8);
  }
  func_0x000148860610(uStack_18 ^ (ulonglong)auStack_d8);
  return;
}


