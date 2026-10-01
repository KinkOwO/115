// sub_142FBB290  size=378

__int64 __fastcall sub_142FBB290(_QWORD *a1, unsigned __int64 a2)
{
  __int64 result; // rax
  __int64 *v5; // rsi
  __int64 *v6; // rax
  __int64 *v7; // rcx
  __int64 v8; // rax
  __int64 v9; // rdi
  __int64 v10; // rbx
  int v11; // eax
  __int128 v12; // [rsp+50h] [rbp-58h] BYREF
  __int128 v13; // [rsp+60h] [rbp-48h]
  void *v14; // [rsp+C0h] [rbp+18h]

  nullsub_1(a1);
  result = sub_141780030(a2);
  if ( result == 2258 )
  {
    v5 = (__int64 *)a1[137];
    v6 = (__int64 *)v5[1];
    *(_QWORD *)&v13 = v6;
    DWORD2(v13) = 0;
    v7 = v5;
    while ( *((_BYTE *)v6 + 25) == 0 )
    {
      *(_QWORD *)&v13 = v6;
      if ( v6[4] >= a2 )
      {
        DWORD2(v13) = 1;
        v7 = v6;
        v6 = (__int64 *)*v6;
      }
      else
      {
        DWORD2(v13) = 0;
        v6 = (__int64 *)v6[2];
      }
    }
    if ( *((_BYTE *)v7 + 25) != 0 || a2 < v7[4] )
    {
      if ( a1[138] == 0x333333333333333LL )
        unknown_libname_7();
      v12 = (unsigned __int64)(a1 + 137);
      __wind
      {
        v8 = sub_146E8BA20(80);
        v9 = v8;
        *((_QWORD *)&v12 + 1) = v8;
      }
      __unwind
      {
        sub_140151060(&v12);
      }
      __wind
      {
        *(_QWORD *)(v8 + 32) = a2;
        v10 = v8 + 40;
        v14 = (void *)(v8 + 40);
        sub_146E9F7D0((void *)(v8 + 40));
        __wind
        {
          *(_DWORD *)(v10 + 32) = 0;
        }
        __unwind
        {
          sub_146E9F800(v14);
        }
        *(_QWORD *)v9 = v5;
        *(_QWORD *)(v9 + 8) = v5;
        *(_QWORD *)(v9 + 16) = v5;
        *(_WORD *)(v9 + 24) = 0;
      }
      __unwind
      {
        sub_1401511D0(&v12);
      }
      __wind
      {
        *((_QWORD *)&v12 + 1) = 0;
      }
      __unwind
      {
        sub_140F5EF20(&v12);
      }
      v12 = v13;
      sub_14014F0E0(a1 + 137, &v12, v9);
    }
    v11 = sub_146E8C7D0(&unk_149CC6290);
    return sub_14538A020((int)a1 - 24, v11, -1, 0, 0, 0, 0, 1);
  }
  return result;
}
