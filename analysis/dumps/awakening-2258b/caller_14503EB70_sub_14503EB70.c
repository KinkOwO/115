// sub_14503EB70  va=0x14503EB70  size=1694

__int64 __fastcall sub_14503EB70(_QWORD *a1, __int64 a2)
{
  __int64 v3; // rdi
  __int64 result; // rax
  int v5; // r13d
  __int64 v6; // rax
  __int64 v7; // rdx
  unsigned int v8; // r15d
  __int64 v9; // rcx
  __int64 v10; // rcx
  __int64 v11; // rax
  unsigned int *v12; // rbx
  int v13; // r9d
  int v14; // r8d
  unsigned int v15; // r12d
  _WORD *v16; // rax
  __int64 v17; // r8
  _WORD *v18; // rax
  char v19; // bl
  unsigned __int64 v20; // rdx
  __int64 v21; // rcx
  unsigned __int64 v22; // rdx
  __int64 v23; // rcx
  unsigned __int64 v24; // rdx
  __int64 v25; // rcx
  unsigned __int64 v26; // rdx
  __int64 v27; // rcx
  __int64 v28; // rax
  __int64 v29; // rcx
  _DWORD *v30; // rbx
  unsigned __int8 v31; // r14
  int v32; // eax
  int v33; // ecx
  unsigned int v34; // eax
  _DWORD *v35; // rcx
  _LocaleUpdate *v36; // rcx
  _QWORD *v37; // rax
  _LocaleUpdate *v38; // rcx
  __int64 v39; // rax
  __int64 v40; // rcx
  __int64 v41; // rax
  __int64 v42; // rcx
  __int64 v43; // rax
  __int64 v44; // rcx
  __int64 v45; // rax
  __int64 v46; // rcx
  __int64 v47; // rax
  __int64 v48; // rcx
  __int64 v49; // rax
  unsigned int v50; // ebx
  unsigned __int16 v51; // r14
  __int64 v52; // r8
  __int64 v53; // rcx
  __int64 v54; // rdx
  __int64 v55; // rax
  __int64 v56; // rax
  __int64 v57; // rcx
  __int64 v58; // rax
  __int64 v59; // rcx
  __int64 v60; // rax
  __int64 v61; // rcx
  int v62; // ebx
  int v63; // eax
  __int64 v64; // rcx
  __int64 v65; // rax
  __int64 v66; // rax
  __int64 v67; // rax
  char v68; // [rsp+38h] [rbp-D0h]
  int v69; // [rsp+3Ch] [rbp-CCh] BYREF
  unsigned int v70; // [rsp+40h] [rbp-C8h]
  int v71; // [rsp+44h] [rbp-C4h] BYREF
  _QWORD *v72; // [rsp+48h] [rbp-C0h]
  _QWORD v73[4]; // [rsp+50h] [rbp-B8h] BYREF
  __int64 v74; // [rsp+70h] [rbp-98h]
  __int64 v75; // [rsp+78h] [rbp-90h]
  __int64 v76; // [rsp+80h] [rbp-88h] BYREF
  char v77; // [rsp+8Dh] [rbp-7Bh]
  _BYTE v78[18]; // [rsp+8Eh] [rbp-7Ah]
  _QWORD v79[2]; // [rsp+A0h] [rbp-68h] BYREF
  __int64 v80; // [rsp+B0h] [rbp-58h]
  unsigned __int64 v81; // [rsp+B8h] [rbp-50h]
  _QWORD v82[2]; // [rsp+C0h] [rbp-48h] BYREF
  __m128i si128; // [rsp+D0h] [rbp-38h]
  _QWORD v84[2]; // [rsp+E8h] [rbp-20h] BYREF
  __m128i v85; // [rsp+F8h] [rbp-10h]
  void *retaddr; // [rsp+140h] [rbp+38h]

  v75 = -2;
  LODWORD(v3) = 0;
  LOBYTE(a2) = 1;
  result = sub_14503D4D0(a1, a2);
  if ( (_BYTE)result != 0 )
  {
    v5 = *(_DWORD *)sub_14503CE30(a1);
    v6 = a1[191];
    if ( v6 != 0 && *(_DWORD *)(v6 + 8) != 0 )
      v7 = a1[192];
    else
      v7 = 0;
    v8 = sub_145AD5C20(qword_14E683C80, v7);
    v70 = sub_14503CDD0(a1);
    v9 = a1[191];
    if ( v9 != 0 && *(_DWORD *)(v9 + 8) != 0 )
      v10 = a1[192];
    else
      v10 = 0;
    v11 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v10 + 152LL))(v10);
    v12 = (unsigned int *)(v11 + 24);
    LOBYTE(v13) = 1;
    LOBYTE(v14) = 50;
    sub_1480A6620(v11 + 24, 4, v14, v13, v11 + 28);
    v15 = *v12;
    v72 = v73;
    if ( byte_14DC67898 != 0 )
    {
      v69 = 424;
      v16 = (_WORD *)sub_146E8C7D0(&unk_14A6DABE0);
      v79[0] = 0;
      v80 = 0;
      v81 = 7;
      v17 = -1;
      do
        ++v17;
      while ( v16[v17] != 0 );
      sub_14014C8D0(v79, v16);
      __eh34_enter_wind_state(-1, 0);
      v18 = (_WORD *)sub_140438530(v84, v79, &v69);
      __eh34_enter_wind_state(0, 1);
      v19 = 3;
      __eh34_enter_wind_state(1, 2);
      __eh34_enter_wind_state(2, 3);
    }
    else
    {
      v69 = 0;
      v76 = 0;
      *(_QWORD *)&v78[2] = 0;
      *(_QWORD *)&v78[10] = 7;
      sub_14014C8D0(&v76, (void *)&Source);
      __eh34_enter_wind_state(-1, 0);
      __eh34_enter_wind_state(0, 1);
      __eh34_enter_wind_state(1, 2);
      v18 = (_WORD *)sub_140438530(v82, &v76, &v69);
      __eh34_enter_wind_state(2, 3);
      v19 = 12;
    }
    v68 = v19;
    v73[0] = 0;
    v73[2] = 0;
    v73[3] = 0;
    qmemcpy(v73, v18, sizeof(v73));
    *((_QWORD *)v18 + 2) = 0;
    *((_QWORD *)v18 + 3) = 7;
    *v18 = 0;
    __wind
    {
      LODWORD(v74) = *((_DWORD *)v18 + 8);
    }
    __unwind
    {
      unknown_libname_4(v72);
    }
    sub_145ACB980(qword_14E683C80, 12, v73);
    if ( __eh34_unwind(3) )
    {
      if ( (v19 & 8) != 0 )
        sub_1401512E0(v82);
      __eh34_continue_unwinding(3, 2);
    }
    __eh34_exit_wind_state(3, 2);
    if ( (v19 & 8) != 0 )
    {
      v19 &= ~8u;
      v68 = v19;
      if ( si128.m128i_i64[1] >= 8uLL )
      {
        v20 = 2 * si128.m128i_i64[1] + 2;
        v21 = v82[0];
        if ( v20 >= 0x1000 )
        {
          v20 = 2 * si128.m128i_i64[1] + 41;
          v21 = *(_QWORD *)(v82[0] - 8LL);
          if ( (unsigned __int64)(v82[0] - v21 - 8) > 0x1F )
            invalid_parameter_noinfo_noreturn();
        }
        j_j_scalable_free(v21, v20);
      }
      si128 = _mm_load_si128((const __m128i *)&xmmword_1491AB7C0);
      LOWORD(v82[0]) = 0;
    }
    if ( __eh34_unwind(2) )
    {
      if ( (v68 & 4) != 0 )
        unknown_libname_4(&v76);
      __eh34_continue_unwinding(2, 1);
    }
    __eh34_exit_wind_state(2, 1);
    if ( (v19 & 4) != 0 )
    {
      v19 &= ~4u;
      v68 = v19;
      if ( *(_QWORD *)&v78[10] >= 8u )
      {
        v22 = 2LL * *(_QWORD *)&v78[10] + 2;
        v23 = v76;
        if ( v22 >= 0x1000 )
        {
          v22 = 2LL * *(_QWORD *)&v78[10] + 41;
          v23 = *(_QWORD *)(v76 - 8);
          if ( (unsigned __int64)(v76 - v23 - 8) > 0x1F )
            invalid_parameter_noinfo_noreturn();
        }
        j_j_scalable_free(v23, v22);
      }
      *(_QWORD *)&v78[2] = 0;
      *(_QWORD *)&v78[10] = 7;
      LOWORD(v76) = 0;
    }
    if ( __eh34_unwind(1) )
    {
      if ( (v68 & 2) != 0 )
        sub_1401512E0(v84);
      __eh34_continue_unwinding(1, 0);
    }
    __eh34_exit_wind_state(1, 0);
    if ( (v19 & 2) != 0 )
    {
      v19 &= ~2u;
      v68 = v19;
      if ( v85.m128i_i64[1] >= 8uLL )
      {
        v24 = 2 * v85.m128i_i64[1] + 2;
        v25 = v84[0];
        if ( v24 >= 0x1000 )
        {
          v24 = 2 * v85.m128i_i64[1] + 41;
          v25 = *(_QWORD *)(v84[0] - 8LL);
          if ( (unsigned __int64)(v84[0] - v25 - 8) > 0x1F )
            invalid_parameter_noinfo_noreturn();
        }
        j_j_scalable_free(v25, v24);
      }
      v85 = _mm_load_si128((const __m128i *)&xmmword_1491AB7C0);
      LOWORD(v84[0]) = 0;
    }
    if ( __eh34_unwind(0) )
    {
      if ( (v68 & 1) != 0 )
        unknown_libname_4(v79);
      __eh34_propagate_exception_into_caller(0, -1);
    }
    __eh34_exit_wind_state(0, -1);
    if ( (v19 & 1) != 0 )
    {
      if ( v81 >= 8 )
      {
        v26 = 2 * v81 + 2;
        v27 = v79[0];
        if ( v26 >= 0x1000 )
        {
          v26 = 2 * v81 + 41;
          v27 = *(_QWORD *)(v79[0] - 8LL);
          if ( (unsigned __int64)(v79[0] - v27 - 8) > 0x1F )
            invalid_parameter_noinfo_noreturn();
        }
        j_j_scalable_free(v27, v26);
      }
      v80 = 0;
      v81 = 7;
      LOWORD(v79[0]) = 0;
    }
    v28 = a1[191];
    if ( v28 != 0 && *(_DWORD *)(v28 + 8) != 0 )
      v29 = a1[192];
    else
      v29 = 0;
    v30 = (_DWORD *)(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v29 + 1048LL))(v29);
    sub_146E920A0(v30, &v69);
    v31 = v69;
    v32 = v69 + *v30 + 196;
    v33 = v30[1];
    if ( v33 != 0 && v32 != 0 && v33 != v32 && retaddr != nullptr )
    {
      sub_146D89B40(retaddr, v30);
      v31 = v69;
    }
    v34 = sub_14503CDD0(a1);
    result = sub_145ABB480(v34);
    if ( result != 0 )
    {
      v35 = (_DWORD *)a1[191];
      if ( v35 != nullptr && v35[2] != 0 )
      {
        v35 = (_DWORD *)a1[192];
        if ( v35 != nullptr
          && *(_BYTE *)(result + 6440) == 1
          && *(_BYTE *)((*(__int64 (__fastcall **)(_DWORD *))(*(_QWORD *)v35 + 1080LL))(v35) + 285) != 0 )
        {
          *(_DWORD *)v78 = -1;
          *(_DWORD *)&v78[7] = -1;
          v77 = 1;
          v78[4] = 0;
          *(_WORD *)&v78[5] = v8;
          v36 = qword_14E6387A8;
          if ( qword_14E6387A8 == nullptr )
          {
            v37 = (_QWORD *)sub_146E8BA20(176);
            v72 = v37;
            __wind
            {
              if ( v37 != nullptr )
                v38 = (_LocaleUpdate *)sub_140B896A0(v37);
              else
                v38 = nullptr;
            }
            __unwind
            {
              j_j_scalable_free(v72, 176);
            }
            qword_14E6387A8 = v38;
            (**(void (__fastcall ***)(_LocaleUpdate *))v38)(v38);
            v36 = qword_14E6387A8;
          }
          sub_140B8AD40((__int64)v36, (__int64)&v76);
        }
      }
      v39 = sub_146D74000(v35);
      sub_146D746E0(v39, 271);
      v41 = sub_146D74000(v40);
      sub_146D76180(v41, (unsigned __int16)v5);
      v43 = sub_146D74000(v42);
      sub_146D75CE0(v43, v70);
      v45 = sub_146D74000(v44);
      sub_146D76180(v45, (unsigned __int16)v8);
      v47 = sub_146D74000(v46);
      sub_146D75CE0(v47, v15);
      v49 = sub_146D74000(v48);
      sub_146D75CC0(v49, v31);
      v50 = 0xFFFF;
      v51 = -1;
      v52 = a1[202];
      v53 = a1[203] - v52;
      v54 = v53 / 40;
      if ( v53 / 40 != 0 )
      {
        v53 = *(_QWORD *)(v52 + 8);
        if ( v53 != 0 )
        {
          v55 = (*(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v53 + 152LL))(v53, v54);
          v50 = sub_140256DC0(v55 + 16);
          v51 = sub_145AD55B0(qword_14E683C80);
        }
      }
      v56 = sub_146D74000(v53);
      sub_146D76180(v56, v51);
      v58 = sub_146D74000(v57);
      sub_146D75CE0(v58, v50);
      v71 = 0;
      v60 = sub_146D74000(v59);
      sub_146D75B10(v60, &v71, 4);
      sub_146D75AF0(v61);
      (*(void (__fastcall **)(_QWORD *))(*a1 + 264LL))(a1);
      v62 = _RTDynamicCast(a1[200], 0, &off_14DCB4FF0, &off_14DCB64C8, 0);
      v63 = sub_14503D130();
      v64 = a1[191];
      if ( v64 != 0 && *(_DWORD *)(v64 + 8) != 0 )
        v3 = a1[192];
      sub_144CCD4B0(v63, v62, v3, v8, v8);
      v65 = sub_14503D130();
      result = sub_144CCCEE0(v65, v15);
      if ( (_BYTE)result == 1 )
      {
        v66 = sub_14503D130();
        sub_140697090(v66, v8);
        v67 = sub_14503D130();
        return sub_1410AF2D0(v67, v15);
      }
    }
  }
  return result;
}
