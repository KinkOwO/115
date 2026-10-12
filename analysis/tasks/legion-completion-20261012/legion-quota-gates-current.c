// CURRENT SHA256 515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb

// FUNCTION 14250d560 FUN_14250d560

int FUN_14250d560(longlong param_1)

{
  return (int)*(short *)(*(longlong *)(param_1 + 0x80) + 0x160);
}



// FUNCTION 14250ccb0 FUN_14250ccb0

int FUN_14250ccb0(longlong param_1)

{
  return (int)*(short *)(*(longlong *)(param_1 + 0x80) + 0x162);
}



// FUNCTION 142ab4ad0 FUN_142ab4ad0

undefined4 * FUN_142ab4ad0(longlong param_1,undefined4 *param_2)

{
  undefined4 uVar1;
  undefined4 uVar2;
  undefined4 uVar3;
  undefined4 uVar4;
  undefined4 uVar5;
  undefined4 uVar6;
  undefined4 uVar7;
  undefined4 uVar8;
  undefined4 *puVar9;
  longlong lVar10;
  undefined4 *puVar11;
  
  lVar10 = 2;
  puVar9 = (undefined4 *)(param_1 + 0x20);
  puVar11 = param_2;
  do {
    uVar1 = puVar9[1];
    uVar2 = puVar9[2];
    uVar3 = puVar9[3];
    uVar4 = puVar9[4];
    uVar5 = puVar9[5];
    uVar6 = puVar9[6];
    uVar7 = puVar9[7];
    *puVar11 = *puVar9;
    puVar11[1] = uVar1;
    puVar11[2] = uVar2;
    puVar11[3] = uVar3;
    uVar1 = puVar9[8];
    uVar2 = puVar9[9];
    uVar3 = puVar9[10];
    uVar8 = puVar9[0xb];
    puVar11[4] = uVar4;
    puVar11[5] = uVar5;
    puVar11[6] = uVar6;
    puVar11[7] = uVar7;
    uVar4 = puVar9[0xc];
    uVar5 = puVar9[0xd];
    uVar6 = puVar9[0xe];
    uVar7 = puVar9[0xf];
    puVar11[8] = uVar1;
    puVar11[9] = uVar2;
    puVar11[10] = uVar3;
    puVar11[0xb] = uVar8;
    uVar1 = puVar9[0x10];
    uVar2 = puVar9[0x11];
    uVar3 = puVar9[0x12];
    uVar8 = puVar9[0x13];
    puVar11[0xc] = uVar4;
    puVar11[0xd] = uVar5;
    puVar11[0xe] = uVar6;
    puVar11[0xf] = uVar7;
    uVar4 = puVar9[0x14];
    uVar5 = puVar9[0x15];
    uVar6 = puVar9[0x16];
    uVar7 = puVar9[0x17];
    puVar11[0x10] = uVar1;
    puVar11[0x11] = uVar2;
    puVar11[0x12] = uVar3;
    puVar11[0x13] = uVar8;
    uVar1 = puVar9[0x18];
    uVar2 = puVar9[0x19];
    uVar3 = puVar9[0x1a];
    uVar8 = puVar9[0x1b];
    puVar11[0x14] = uVar4;
    puVar11[0x15] = uVar5;
    puVar11[0x16] = uVar6;
    puVar11[0x17] = uVar7;
    uVar4 = puVar9[0x1c];
    uVar5 = puVar9[0x1d];
    uVar6 = puVar9[0x1e];
    uVar7 = puVar9[0x1f];
    puVar11[0x18] = uVar1;
    puVar11[0x19] = uVar2;
    puVar11[0x1a] = uVar3;
    puVar11[0x1b] = uVar8;
    puVar11[0x1c] = uVar4;
    puVar11[0x1d] = uVar5;
    puVar11[0x1e] = uVar6;
    puVar11[0x1f] = uVar7;
    lVar10 = lVar10 + -1;
    puVar9 = puVar9 + 0x20;
    puVar11 = puVar11 + 0x20;
  } while (lVar10 != 0);
  return param_2;
}



// FUNCTION 14250bc60 FUN_14250bc60

int FUN_14250bc60(undefined8 param_1,int *param_2)

{
  return *param_2 + -0x65;
}


