// sub_14620CF30  size=1267

__int64 __fastcall sub_14620CF30(unsigned int *a1, int a2, int a3, int a4, __int64 a5)
{
  __int64 result; // rax
  __int64 v10; // rdx
  _QWORD *v11; // rbx
  __int64 v12; // rax
  __int64 v13; // rax
  __int64 v14; // r8
  __int64 v15; // rax
  int v16; // eax
  _WORD *v17; // rax
  _QWORD *v18; // rcx
  __int64 v19; // r8
  _WORD *v20; // rax
  _QWORD *v21; // rcx
  __int64 v22; // rbx
  __int64 v23; // r8
  _WORD *v24; // rax
  __int64 v25; // rax
  char *v26; // rbx
  char *v27; // rdi
  unsigned __int64 v28; // rdx
  char *v29; // rax
  unsigned __int64 v30; // rdx
  __int64 v31; // rcx
  __int64 v32; // rax
  _QWORD *v33; // rbx
  _QWORD v34[2]; // [rsp+20h] [rbp-81h] BYREF
  _QWORD v35[2]; // [rsp+30h] [rbp-71h] BYREF
  __m128i si128; // [rsp+40h] [rbp-61h]
  __int64 v37; // [rsp+50h] [rbp-51h]
  int v38; // [rsp+58h] [rbp-49h]
  int v39; // [rsp+5Ch] [rbp-45h]
  int v40; // [rsp+60h] [rbp-41h]
  int v41; // [rsp+64h] [rbp-3Dh]
  void *v42; // [rsp+68h] [rbp-39h] BYREF
  void *v43[2]; // [rsp+70h] [rbp-31h]
  char v44; // [rsp+80h] [rbp-21h]
  __int64 v45; // [rsp+88h] [rbp-19h]
  int v46; // [rsp+90h] [rbp-11h]

  v34[1] = -2;
  result = (*(__int64 (__fastcall **)(unsigned int *))(*(_QWORD *)a1 + 1392LL))(a1);
  if ( (_BYTE)result != 0 )
  {
    v10 = *((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer + (unsigned int)TlsIndex);
    if ( dword_14EF2E9A0 > *(_DWORD *)(v10 + 420620) )
    {
      Init_thread_header(&dword_14EF2E9A0, v10, 420620);
      if ( dword_14EF2E9A0 == -1 )
      {
        __wind
        {
          qword_14EF2E960 = 0;
          dword_14EF2E968 = 0;
          v34[0] = &qword_14EF2E990;
          qword_14EF2E990 = 0;
          qword_14EF2E998 = 0;
          v32 = sub_146E8BA20(32);
          *(_QWORD *)v32 = v32;
          *(_QWORD *)(v32 + 8) = v32;
          *(_QWORD *)(v32 + 16) = v32;
          *(_WORD *)(v32 + 24) = 257;
          qword_14EF2E990 = v32;
          __wind
          {
            dword_14EF2E96C = 4;
            xmmword_14EF2E970 = 0;
            qword_14EF2E980 = 0;
            v33 = (_QWORD *)qword_14EF2E990;
            sub_140178C60(&qword_14EF2E990, &qword_14EF2E990, *(_QWORD *)(qword_14EF2E990 + 8));
            v33[1] = v33;
            *v33 = v33;
            v33[2] = v33;
            qword_14EF2E998 = 0;
            dword_14EF2E988 = 1065353216;
          }
          __unwind
          {
            sub_140178FB0(&qword_14EF2E990);
          }
          atexit(sub_14903F560);
        }
        __unwind
        {
          Init_thread_abort(&dword_14EF2E9A0);
        }
        Init_thread_footer(&dword_14EF2E9A0);
      }
    }
    dword_14EF2E96C = 4;
    xmmword_14EF2E970 = 0;
    qword_14EF2E980 = 0;
    v11 = (_QWORD *)qword_14EF2E990;
    sub_140178C60(&qword_14EF2E990, &qword_14EF2E990, *(_QWORD *)(qword_14EF2E990 + 8));
    v11[1] = v11;
    *v11 = v11;
    v11[2] = v11;
    qword_14EF2E998 = 0;
    dword_14EF2E988 = 1065353216;
    LODWORD(qword_14EF2E960) = a2;
    HIDWORD(qword_14EF2E960) = a3;
    dword_14EF2E968 = a4;
    dword_14EF2E96C = (*(__int64 (__fastcall **)(unsigned int *))(*(_QWORD *)a1 + 736LL))(a1);
    *(_QWORD *)&xmmword_14EF2E970 = a5;
    v12 = (*(__int64 (__fastcall **)(unsigned int *))(*(_QWORD *)a1 + 1336LL))(a1);
    v13 = sub_1450BE4E0(v12);
    *((_QWORD *)&xmmword_14EF2E970 + 1) = v13;
    if ( v13 != 0 )
    {
      LOBYTE(v14) = 1;
      v15 = (*(__int64 (__fastcall **)(__int64, _QWORD, __int64))(*(_QWORD *)v13 + 8264LL))(v13, a1[2971], v14);
      v16 = sub_145F73510(v15);
    }
    else
    {
      v16 = (*(__int64 (__fastcall **)(unsigned int *))(*(_QWORD *)a1 + 1504LL))(a1);
    }
    LODWORD(qword_14EF2E980) = v16;
    v35[0] = 0;
    si128 = _mm_load_si128((const __m128i *)&xmmword_1491AB7C0);
    sub_14014C8D0(v35, (void *)&Source);
    __wind
    {
      v22 = -1;
      v37 = -1;
      v38 = 1065353216;
      v39 = 1;
      v40 = 1;
      v41 = 2;
      v42 = nullptr;
      *(_OWORD *)v43 = 0;
      __wind
      {
        v44 = 1;
        v45 = 0;
        v46 = 1;
      }
      __unwind
      {
        sub_140161230(&v42);
      }
    }
    __unwind
    {
      unknown_libname_4(v35);
    }
    __wind
    {
      if ( (_QWORD)xmmword_14EF2E970 != 0
        && ((unsigned __int8)sub_145D742A0(xmmword_14EF2E970, 7) != 0
         || (unsigned __int8)sub_145D742A0(xmmword_14EF2E970, 8) != 0
         || (unsigned __int8)sub_145D742A0(xmmword_14EF2E970, 9) != 0) )
      {
        v17 = (_WORD *)sub_146E8C7D0(&unk_1491C5A08);
        v34[0] = v17;
        v18 = v43[0];
        if ( v43[0] == v43[1] )
        {
          sub_1405A1990(&v42, v43[0], v34);
        }
        else
        {
          *(_QWORD *)v43[0] = 0;
          v18[2] = 0;
          v18[3] = 7;
          *(_WORD *)v18 = 0;
          v19 = -1;
          do
            ++v19;
          while ( v17[v19] != 0 );
          sub_14014C8D0(v18, v17);
          v43[0] = (char *)v43[0] + 32;
        }
        v20 = (_WORD *)sub_146E8C7D0(&unk_1491C5AD0);
        v34[0] = v20;
        v21 = v43[0];
        if ( v43[0] == v43[1] )
        {
          sub_1405A1990(&v42, v43[0], v34);
        }
        else
        {
          *(_QWORD *)v43[0] = 0;
          v21[2] = 0;
          v21[3] = 7;
          *(_WORD *)v21 = 0;
          do
            ++v22;
          while ( v20[v22] != 0 );
          sub_14014C8D0(v21, v20);
          v43[0] = (char *)v43[0] + 32;
        }
      }
      else
      {
        v24 = (_WORD *)sub_146E8C7D0(&unk_1491C5A08);
        do
          ++v22;
        while ( v24[v22] != 0 );
        sub_14014C8D0(v35, v24);
      }
      if ( *((_QWORD *)&xmmword_14EF2E970 + 1) != 0 )
      {
        LOBYTE(v23) = 1;
        v25 = (*(__int64 (__fastcall **)(_QWORD, _QWORD, __int64))(**((_QWORD **)&xmmword_14EF2E970 + 1) + 8264LL))(
                *((_QWORD *)&xmmword_14EF2E970 + 1),
                a1[2971],
                v23);
        HIDWORD(qword_14EF2E980) = sub_145EC9860(v25, v35);
      }
      result = sub_14620D430(&qword_14EF2E960);
    }
    __unwind
    {
      sub_1401DDB40(v35);
    }
    __wind
    {
      v26 = (char *)v42;
      if ( v42 != nullptr )
      {
        v27 = (char *)v43[0];
        if ( v42 != v43[0] )
        {
          do
          {
            unknown_libname_4(v26);
            v26 += 32;
          }
          while ( v26 != v27 );
          v26 = (char *)v42;
        }
        v28 = ((char *)v43[1] - (char *)v26) & 0xFFFFFFFFFFFFFFE0uLL;
        v29 = v26;
        if ( v28 >= 0x1000 )
        {
          v28 += 39LL;
          v26 = *((char **)v26 - 1);
          if ( (unsigned __int64)(v29 - v26 - 8) > 0x1F )
            invalid_parameter_noinfo_noreturn();
        }
        result = j_j_scalable_free(v26, v28);
        v42 = nullptr;
        *(_OWORD *)v43 = 0;
      }
    }
    __unwind
    {
      unknown_libname_4(v35);
    }
    if ( si128.m128i_i64[1] >= 8uLL )
    {
      v30 = 2 * si128.m128i_i64[1] + 2;
      v31 = v35[0];
      if ( v30 >= 0x1000 )
      {
        v30 = 2 * si128.m128i_i64[1] + 41;
        v31 = *(_QWORD *)(v35[0] - 8LL);
        if ( (unsigned __int64)(v35[0] - v31 - 8) > 0x1F )
          invalid_parameter_noinfo_noreturn();
      }
      result = j_j_scalable_free(v31, v30);
    }
    si128 = _mm_load_si128((const __m128i *)&xmmword_1491AB7C0);
    LOWORD(v35[0]) = 0;
  }
  return result;
}
