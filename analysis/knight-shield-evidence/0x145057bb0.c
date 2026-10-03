char __fastcall sub_145057BB0(__int64 a1, __int64 a2, unsigned int a3)
{
  __int64 v5; // rcx
  __int64 v6; // rbp
  __int64 v7; // rax
  __int64 v8; // r8
  __int64 v9; // rax
  __int64 v10; // rsi
  __int64 v11; // rdx
  __int64 v12; // rcx
  __int64 v13; // r8
  __int64 v14; // rax
  __int64 v15; // rax
  __int128 v17; // [rsp+30h] [rbp-48h] BYREF
  __int128 v18; // [rsp+40h] [rbp-38h]
  __int64 v19; // [rsp+88h] [rbp+10h] BYREF

  v19 = a2; /*0x145057bb3*/
  if ( a3 > 4 ) /*0x145057bd9*/
    return 0; /*0x145057bd9*/
  *(_DWORD *)(*(_QWORD *)sub_1416DBCA0(a1: a1 + 368, a2: &v17, a3: &v19) + 36LL) = a3; /*0x145057bf6*/
  v6 = *(_QWORD *)(a1 + 384); /*0x145057c00*/
  v7 = *(_QWORD *)(v6 + 8); /*0x145057c03*/
  *(_QWORD *)&v18 = v7; /*0x145057c07*/
  DWORD2(v18) = 0; /*0x145057c0f*/
  v8 = v6; /*0x145057c14*/
  while ( *(_BYTE *)(v7 + 25) == 0 ) /*0x145057c1b*/
  {
    *(_QWORD *)&v18 = v7; /*0x145057c20*/
    if ( *(_DWORD *)(v7 + 28) >= (signed int)a3 ) /*0x145057c28*/
    {
      DWORD2(v18) = 1; /*0x145057c35*/
      v8 = v7; /*0x145057c3d*/
      v7 = *(_QWORD *)v7; /*0x145057c40*/
    }
    else
    {
      DWORD2(v18) = 0; /*0x145057c2a*/
      v7 = *(_QWORD *)(v7 + 16); /*0x145057c2f*/
    }
  }
  if ( *(_BYTE *)(v8 + 25) != 0 || (signed int)a3 < *(_DWORD *)(v8 + 28) ) /*0x145057c53*/
  {
    if ( *(_QWORD *)(a1 + 392) == 0x555555555555555LL ) /*0x145057c63*/
      unknown_libname_7(a1: v5); /*0x145057d5b*/
    v17 = (unsigned __int64)(a1 + 384); /*0x145057c69*/
    __wind /*0xf1c000000000002c*/
    {
      *((_QWORD *)&v17 + 1) = 0; /*0x145057c73*/
      v9 = sub_146E8BA20(a1: 48); /*0x145057c7d*/
      *((_QWORD *)&v17 + 1) = v9; /*0x145057c82*/
    }
    __unwind
    {
      sub_14014EE00(a1: &v17); /*0x148b8d577*/
    }
    __wind /*0xf1c0000000000044*/
    {
      *(_DWORD *)(v9 + 28) = a3; /*0x145057c87*/
      *(_DWORD *)(v9 + 32) = 1; /*0x145057c8a*/
      *(_DWORD *)(v9 + 36) = -1; /*0x145057c91*/
      *(_BYTE *)(v9 + 40) = 0; /*0x145057c98*/
      *(_QWORD *)v9 = v6; /*0x145057c9c*/
      *(_QWORD *)(v9 + 8) = v6; /*0x145057c9f*/
      *(_QWORD *)(v9 + 16) = v6; /*0x145057ca3*/
      *(_WORD *)(v9 + 24) = 0; /*0x145057ca7*/
    }
    __unwind
    {
      sub_14014EEA0(a1: &v17); /*0x148b8d567*/
    }
    __wind /*0xf1c000000000005c*/
    {
      *((_QWORD *)&v17 + 1) = 0; /*0x145057cad*/
    }
    __unwind
    {
      sub_14014EE70(a1: &v17); /*0x148b8d557*/
    }
    v17 = v18; /*0x145057cb7*/
    v8 = sub_14014F0E0(a1: a1 + 384, a2: &v17, a3: v9); /*0x145057ccc*/
  }
  *(_BYTE *)(v8 + 40) = 0; /*0x145057ccf*/
  if ( *(_QWORD *)(v8 + 32) == v19 ) /*0x145057ce0*/
    return 0; /*0x145057d44*/
  *(_QWORD *)(v8 + 32) = v19; /*0x145057cef*/
  v10 = sub_145058440(a1, a2: (unsigned int)v19, a3: HIDWORD(v19)); /*0x145057d15*/
  v14 = sub_145EFAFB0(a1: v12, a2: v11, a3: v13); /*0x145057d18*/
  v15 = sub_1450BE540(a1: v14); /*0x145057d20*/
  if ( v15 != 0 ) /*0x145057d28*/
    sub_144BE6710(a1: v15, a2: a3, a3: *(unsigned int *)(v10 + 8)); /*0x145057d33*/
  *(_BYTE *)(a1 + 120) |= a3 == 0; /*0x145057d3d*/
  return 1; /*0x145057d46*/
}
