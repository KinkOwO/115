// sub_140B8FE20  va=0x140B8FE20  size=500

void __fastcall sub_140B8FE20(void *a1)
{
  __int64 v2; // rbx
  __int64 v3; // rax
  void *v4; // rax
  void *v5; // rbp
  _QWORD *v6; // rsi
  __int64 v7; // rbx
  __int64 v8; // r15
  __int64 v9; // rax
  __int64 v10; // rcx
  __int64 v11; // rax
  __int64 v12; // rdi
  __int64 v13; // rbx
  __int64 v14; // [rsp+20h] [rbp-B8h]
  void *v15; // [rsp+20h] [rbp-B8h]
  _QWORD *v16; // [rsp+28h] [rbp-B0h] BYREF
  __int64 v17; // [rsp+30h] [rbp-A8h]
  __int64 v18; // [rsp+38h] [rbp-A0h]
  __int128 v19; // [rsp+40h] [rbp-98h] BYREF
  __int64 v20; // [rsp+50h] [rbp-88h]
  __int64 v21; // [rsp+60h] [rbp-78h]
  void *v22; // [rsp+68h] [rbp-70h]
  _BYTE v23[32]; // [rsp+70h] [rbp-68h] BYREF
  _BYTE v24[24]; // [rsp+90h] [rbp-48h] BYREF
  void *v25; // [rsp+A8h] [rbp-30h]

  v21 = -2;
  v25 = a1;
  __wind
  {
    v2 = qword_14E6399A0;
    if ( qword_14E6399A0 == 0 )
    {
      v3 = sub_146E8BA20(1360);
      v14 = v3;
      __wind
      {
        if ( v3 != 0 )
          v2 = sub_140B8D280(v3);
        else
          v2 = 0;
      }
      __unwind
      {
        j_j_scalable_free(v14, 1360);
      }
      qword_14E6399A0 = v2;
    }
    sub_14014C810(v23, a1);
    v5 = v4;
    v22 = v4;
    __wind
    {
      v6 = (_QWORD *)(v2 + 952);
      sub_140150980(v2 + 952, &v19, v4);
      v7 = v20;
      if ( *(_BYTE *)(v20 + 25) != 0 || (unsigned __int8)sub_140151510(v6, v5, v20 + 32) != 0 )
        v7 = *v6;
      v8 = *v6;
      if ( v7 == *v6 )
      {
        v9 = sub_140150980(v6, v24, v5);
        v19 = *(_OWORD *)v9;
        v18 = *(_QWORD *)(v9 + 16);
        if ( *(_BYTE *)(v18 + 25) != 0 || (unsigned __int8)sub_140151510(v6, v5, v18 + 32) != 0 )
        {
          if ( v6[1] == 0x38E38E38E38E38ELL )
            unknown_libname_7(v10);
          v16 = v6;
          v17 = 0;
          __wind
          {
            v11 = sub_146E8BA20(72);
            v12 = v11;
            v17 = v11;
          }
          __unwind
          {
            sub_140155C90(&v16);
          }
          __wind
          {
            v13 = v11 + 32;
            v15 = (void *)(v11 + 32);
            sub_14014C810((void *)(v11 + 32), v5);
            __wind
            {
              *(_DWORD *)(v13 + 32) = 1;
            }
            __unwind
            {
              unknown_libname_4(v15);
            }
            *(_QWORD *)v12 = v8;
            *(_QWORD *)(v12 + 8) = v8;
            *(_QWORD *)(v12 + 16) = v8;
            *(_WORD *)(v12 + 24) = 0;
          }
          __unwind
          {
            sub_140156280(&v16);
          }
          __wind
          {
            v17 = 0;
          }
          __unwind
          {
            sub_1401560F0(&v16);
          }
          sub_14014F0E0(v6, &v19, v12);
        }
      }
      else
      {
        ++*(_DWORD *)(v7 + 64);
      }
    }
    __unwind
    {
      unknown_libname_4(v22);
    }
    unknown_libname_4(v5);
  }
  __unwind
  {
    unknown_libname_4(v25);
  }
  unknown_libname_4(a1);
}
