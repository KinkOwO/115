__int64 __fastcall sub_1444E8030(__int64 *a1, __int64 a2, unsigned int *a3)
{
  __int64 v6; // rdi
  __int64 v7; // rdx
  __int64 v8; // r8
  __int64 v9; // rcx
  __int64 v10; // rbx
  __int64 v11; // rax
  char v12; // di
  __int128 v14; // [rsp+30h] [rbp-38h] BYREF
  __int128 v15; // [rsp+40h] [rbp-28h]

  v6 = *a1;
  *(_QWORD *)&v14 = a1;
  v8 = sub_146E8BA20(56);
  *((_QWORD *)&v14 + 1) = v8;
  v9 = *a3;
  *(_DWORD *)(v8 + 28) = v9;
  *(_OWORD *)(v8 + 32) = *(_OWORD *)(a3 + 1);
  *(_QWORD *)(v8 + 48) = *(_QWORD *)(a3 + 5);
  *(_QWORD *)v8 = v6;
  *(_QWORD *)(v8 + 8) = v6;
  *(_QWORD *)(v8 + 16) = v6;
  *(_WORD *)(v8 + 24) = 0;
  v10 = *a1;
  v11 = *(_QWORD *)(*a1 + 8);
  *(_QWORD *)&v15 = v11;
  DWORD2(v15) = 0;
  if ( !*(_BYTE *)(v11 + 25) )
  {
    v9 = *(unsigned int *)(v8 + 28);
    do
    {
      *(_QWORD *)&v15 = v11;
      if ( *(_DWORD *)(v11 + 28) >= (int)v9 )
      {
        DWORD2(v15) = 1;
        v10 = v11;
        v11 = *(_QWORD *)v11;
      }
      else
      {
        DWORD2(v15) = 0;
        v11 = *(_QWORD *)(v11 + 16);
      }
    }
    while ( !*(_BYTE *)(v11 + 25) );
  }
  if ( *(_BYTE *)(v10 + 25) || *(_DWORD *)(v8 + 28) < *(_DWORD *)(v10 + 28) )
  {
    if ( a1[1] == 0x492492492492492LL )
      sub_14014F360(v9, v7);
    v14 = v15;
    v10 = sub_14014F0E0(a1, &v14, v8);
    v12 = 1;
  }
  else
  {
    v12 = 0;
    sub_146E9F3A0(v8, 56);
  }
  *(_QWORD *)a2 = v10;
  *(_BYTE *)(a2 + 8) = v12;
  return a2;
}
