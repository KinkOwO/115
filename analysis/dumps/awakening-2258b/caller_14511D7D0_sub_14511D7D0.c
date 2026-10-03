// sub_14511D7D0  va=0x14511D7D0  size=3534

__int64 __fastcall sub_14511D7D0(__int64 a1, int a2)
{
  __int64 v4; // rdi
  __int64 result; // rax
  __int64 v6; // rbx
  __int64 v7; // rax
  void *v8; // rax
  __int64 v9; // rcx
  __int64 v10; // rbx
  __int64 v11; // rax
  __int64 v12; // rax
  void **v13; // rbx
  __int64 v14; // rax
  __int64 v15; // rax
  _WORD *v16; // rax
  __int64 v17; // r8
  __int64 v18; // r15
  unsigned __int64 v19; // rsi
  void **v20; // r14
  __int64 v21; // rbx
  unsigned __int64 v22; // rdx
  __int64 v23; // rcx
  unsigned __int64 v24; // rdx
  __int64 v25; // rcx
  unsigned __int64 v26; // rdx
  void *v27; // rcx
  unsigned __int64 v28; // rdx
  void *v29; // rcx
  __int64 v30; // rax
  __int64 v31; // rdx
  int v32; // ebx
  __int64 v33; // rax
  __int64 v34; // rdx
  unsigned int v35; // r15d
  __int64 v36; // rax
  __int64 v37; // rbx
  __int64 v38; // rax
  __int64 v39; // rax
  __int64 v40; // r14
  int v41; // ebx
  _WORD *v42; // rax
  __int64 v43; // r8
  __int64 v44; // rax
  char v45; // bl
  unsigned __int64 v46; // rdx
  __int64 v47; // rcx
  unsigned __int64 v48; // rdx
  void *v49; // rcx
  unsigned __int64 v50; // rdx
  __int64 v51; // rcx
  unsigned __int64 v52; // rdx
  __int64 v53; // rcx
  unsigned int v54; // r13d
  __int64 v55; // rcx
  __int64 v56; // rcx
  __int64 v57; // rax
  unsigned int v58; // r12d
  __int64 v59; // rcx
  __int64 v60; // rcx
  _DWORD *v61; // rbx
  unsigned __int8 v62; // r14
  int v63; // eax
  int v64; // ecx
  unsigned int v65; // eax
  _DWORD *v66; // rcx
  _LocaleUpdate *v67; // rcx
  void *v68; // rax
  _LocaleUpdate *v69; // rcx
  __int64 v70; // rax
  __int64 v71; // rcx
  __int64 v72; // rax
  __int64 v73; // rcx
  __int64 v74; // rax
  __int64 v75; // rcx
  __int64 v76; // rax
  __int64 v77; // rcx
  __int64 v78; // rax
  __int64 v79; // rcx
  __int64 v80; // rax
  __int64 v81; // rcx
  __int64 v82; // rax
  __int64 v83; // rcx
  __int64 v84; // rax
  __int64 i; // rcx
  unsigned __int64 v86; // r8
  __int64 v87; // rax
  __int64 v88; // rcx
  int v89; // ebx
  int v90; // eax
  __int64 v91; // rcx
  __int64 v92; // rax
  __int64 v93; // rax
  __int64 v94; // rax
  __int64 v95; // rbx
  __int64 v96; // rax
  char v97; // [rsp+58h] [rbp-B0h]
  _BYTE *v98; // [rsp+60h] [rbp-A8h] BYREF
  int v99; // [rsp+68h] [rbp-A0h] BYREF
  __int128 v100; // [rsp+70h] [rbp-98h] BYREF
  __int128 v101; // [rsp+80h] [rbp-88h]
  int v102; // [rsp+90h] [rbp-78h]
  _BYTE *v103; // [rsp+98h] [rbp-70h]
  void *v104[3]; // [rsp+A0h] [rbp-68h] BYREF
  _QWORD *v105; // [rsp+B8h] [rbp-50h]
  _BYTE *v106; // [rsp+C0h] [rbp-48h]
  _QWORD *v107; // [rsp+C8h] [rbp-40h]
  _QWORD *v108; // [rsp+D0h] [rbp-38h]
  _BYTE v109[56]; // [rsp+D8h] [rbp-30h] BYREF
  __int64 v110; // [rsp+110h] [rbp+8h]
  _BYTE v111[56]; // [rsp+118h] [rbp+10h] BYREF
  __int64 v112; // [rsp+150h] [rbp+48h]
  _BYTE v113[120]; // [rsp+158h] [rbp+50h] BYREF
  void *Src[2]; // [rsp+1D0h] [rbp+C8h] BYREF
  unsigned __int64 v115; // [rsp+1E0h] [rbp+D8h]
  unsigned __int64 v116; // [rsp+1E8h] [rbp+E0h]
  _QWORD v117[4]; // [rsp+1F0h] [rbp+E8h] BYREF
  __int64 v118; // [rsp+218h] [rbp+110h] BYREF
  _QWORD v119[2]; // [rsp+220h] [rbp+118h] BYREF
  __int64 v120; // [rsp+230h] [rbp+128h]
  unsigned __int64 v121; // [rsp+238h] [rbp+130h]
  int v122; // [rsp+240h] [rbp+138h]
  int v123; // [rsp+244h] [rbp+13Ch]
  int v124; // [rsp+248h] [rbp+140h]
  _QWORD v125[2]; // [rsp+250h] [rbp+148h] BYREF
  __int64 v126; // [rsp+260h] [rbp+158h]
  unsigned __int64 v127; // [rsp+268h] [rbp+160h]
  int v128; // [rsp+270h] [rbp+168h]
  int v129; // [rsp+274h] [rbp+16Ch]
  __int128 v130; // [rsp+278h] [rbp+170h]
  __int64 v131; // [rsp+288h] [rbp+180h]
  _QWORD v132[2]; // [rsp+298h] [rbp+190h] BYREF
  __m128i si128; // [rsp+2A8h] [rbp+1A0h]
  _QWORD *v134; // [rsp+2D0h] [rbp+1C8h]
  _QWORD v135[2]; // [rsp+2D8h] [rbp+1D0h] BYREF
  __m128i v136; // [rsp+2E8h] [rbp+1E0h]
  _QWORD *v137; // [rsp+310h] [rbp+208h]
  void *retaddr; // [rsp+350h] [rbp+248h]

  v104[2] = (void *)-2LL;
  LODWORD(v4) = 0;
  result = sub_1459A90F0(qword_14E66C090);
  if ( (_DWORD)result != 1 )
  {
    v6 = *(_QWORD *)(a1 + 848);
    v7 = sub_14723C170(*(unsigned int *)(a1 + 1728));
    sub_14668C520(v6, 2875, v7, 0);
    v8 = (void *)sub_146E8C7D0(&unk_14A6DAC88);
    sub_145A31380(v8, -1, -1, 0);
    return (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 264LL))(a1);
  }
  if ( *(_QWORD *)(a1 + 1432) != 0 )
  {
    if ( (int)sub_14511AF50(a1) < a2 )
    {
      v9 = *(unsigned int *)(a1 + 1736);
LABEL_6:
      v10 = *(_QWORD *)(a1 + 848);
      v11 = sub_14723C170(v9);
      return sub_14668C520(v10, 2875, v11, 0);
    }
    if ( *(_BYTE *)(a1 + 1776) == 0 )
    {
      Src[0] = nullptr;
      v115 = 0;
      v116 = 7;
      __eh34_enter_wind_state(-1, 0);
      v12 = (*(__int64 (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 1432) + 152LL))(*(_QWORD *)(a1 + 1432));
      sub_140256DC0(v12 + 16);
      if ( (unsigned __int8)sub_145F030C0(Src) != 0 )
      {
        v13 = Src;
        if ( v116 >= 8 )
          v13 = (void **)Src[0];
        v14 = sub_14723C170(31472);
        v15 = sub_146E8CF20(v104, v14, v13);
        __wind
        {
          v16 = (_WORD *)sub_14014F430(v15);
          v17 = -1;
          do
            ++v17;
          while ( v16[v17] != 0 );
          sub_14014C8D0(Src, v16);
        }
        __unwind
        {
          sub_140156DE0(v104);
        }
        sub_146E8C910(v104);
      }
      if ( v115 != 0 )
      {
        result = sub_14667BD40(*(_QWORD *)&qword_14E683C78, 232, 0xFFFFFFFFLL);
        if ( result == 0 )
        {
          v103 = v113;
          v118 = -1;
          v119[0] = 0;
          v120 = 0;
          v121 = 7;
          sub_14014C8D0(v119, (void *)&Source);
          __wind
          {
            v122 = 2;
            v123 = dword_14E64D628;
            v124 = dword_14E64D62C;
            v125[0] = 0;
            v126 = 0;
            v127 = 7;
            sub_14014C8D0(v125, (void *)&Source);
            __wind
            {
              v128 = 134;
              v129 = 1;
              v130 = 0;
              v131 = 0;
            }
            __unwind
            {
              unknown_libname_4(v125);
            }
          }
          __unwind
          {
            unknown_libname_4(v119);
          }
          __wind
          {
            v18 = sub_140256CD0(v113, &v118);
            __wind
            {
              v98 = v109;
              v110 = 0;
              __wind
              {
                v105 = v135;
                v137 = nullptr;
                __wind
                {
                  v135[0] = std::_Func_impl_no_alloc<`EmacncipateNewWindow::sendUpdatePacket'::`18'::_lambda_2_,control_event::XW4TYPE::Z::AXH * const,__int64,__int64>::`vftable';
                  v137 = v135;
                }
                __unwind
                {
                  sub_1401EDFD0(v105);
                }
                __wind
                {
                  v106 = v111;
                  v112 = 0;
                  __wind
                  {
                    v107 = v117;
                    memset(v117, 0, 24);
                    __wind
                    {
                      v108 = v132;
                      v134 = nullptr;
                      __wind
                      {
                        v132[0] = std::_Func_impl_no_alloc<`EmacncipateNewWindow::sendUpdatePacket'::`18'::_lambda_1_,control_event::XW4TYPE::Z::AXH * const,__int64,__int64>::`vftable';
                        v134 = v132;
                      }
                      __unwind
                      {
                        sub_1401EDFD0(v108);
                      }
                      __wind
                      {
                        v104[0] = &v100;
                        *(_QWORD *)&v100 = 0;
                        v101 = 0u;
                        v19 = v115;
                        v20 = Src;
                        if ( v116 >= 8 )
                          v20 = (void **)Src[0];
                        if ( v115 >= 8 )
                        {
                          v21 = v115 | 7;
                          if ( (v115 | 7) > 0x7FFFFFFFFFFFFFFELL )
                            v21 = 0x7FFFFFFFFFFFFFFELL;
                          *(_QWORD *)&v100 = sub_14014CB50(&v100, v21 + 1);
                          memmove((void *)v100, v20, 2 * v19 + 2);
                        }
                        else
                        {
                          v100 = *(_OWORD *)v20;
                          v21 = 7;
                        }
                        *(_QWORD *)&v101 = v19;
                        *((_QWORD *)&v101 + 1) = v21;
                      }
                      __unwind
                      {
                        sub_1401EE860(v108);
                      }
                    }
                    __unwind
                    {
                      sub_1401EF1A0(v107);
                    }
                  }
                  __unwind
                  {
                    sub_1401EE860(v106);
                  }
                }
                __unwind
                {
                  sub_1401EE860(v105);
                }
              }
              __unwind
              {
                sub_1401EE860(v98);
              }
            }
            __unwind
            {
              sub_1401F0E60(v103);
            }
            result = sub_14668DCB0(*(_QWORD *)&qword_14E683C78, 232, &v100, v132, v117, v111, v135, v109, v18);
          }
          __unwind
          {
            sub_1401F0E60(&v118);
          }
          __wind
          {
            if ( v127 >= 8 )
            {
              v22 = 2 * v127 + 2;
              v23 = v125[0];
              if ( v22 >= 0x1000 )
              {
                v22 = 2 * v127 + 41;
                v23 = *(_QWORD *)(v125[0] - 8LL);
                if ( (unsigned __int64)(v125[0] - v23 - 8) > 0x1F )
                  invalid_parameter_noinfo_noreturn();
              }
              result = j_j_scalable_free(v23, v22);
            }
            v126 = 0;
            v127 = 7;
            LOWORD(v125[0]) = 0;
          }
          __unwind
          {
            unknown_libname_4(v119);
          }
          if ( v121 >= 8 )
          {
            v24 = 2 * v121 + 2;
            v25 = v119[0];
            if ( v24 >= 0x1000 )
            {
              v24 = 2 * v121 + 41;
              v25 = *(_QWORD *)(v119[0] - 8LL);
              if ( (unsigned __int64)(v119[0] - v25 - 8) > 0x1F )
                invalid_parameter_noinfo_noreturn();
            }
            result = j_j_scalable_free(v25, v24);
          }
          v120 = 0;
          v121 = 7;
          LOWORD(v119[0]) = 0;
        }
        if ( __eh34_unwind(0) )
          goto unwind_state_0;
        __eh34_exit_wind_state(0, -1);
        if ( v116 < 8 )
          goto LABEL_41;
        v26 = 2 * v116 + 2;
        v27 = Src[0];
        if ( v26 < 0x1000
          || (v26 = 2 * v116 + 41,
              v27 = *((void **)Src[0] - 1),
              (unsigned __int64)((char *)Src[0] - (char *)v27 - 8) <= 0x1F) )
        {
          result = j_j_scalable_free(v27, v26);
LABEL_41:
          v115 = 0;
          v116 = 7;
          LOWORD(Src[0]) = 0;
          return result;
        }
LABEL_136:
        invalid_parameter_noinfo_noreturn();
      }
      if ( __eh34_unwind(0) )
      {
unwind_state_0:
        unknown_libname_4(Src);
        __eh34_propagate_exception_into_caller(0, -1);
      }
      __eh34_exit_wind_state(0, -1);
      if ( v116 >= 8 )
      {
        v28 = 2 * v116 + 2;
        v29 = Src[0];
        if ( v28 >= 0x1000 )
        {
          v28 = 2 * v116 + 41;
          v29 = *((void **)Src[0] - 1);
          if ( (unsigned __int64)((char *)Src[0] - (char *)v29 - 8) > 0x1F )
            goto LABEL_136;
        }
        j_j_scalable_free(v29, v28);
      }
      v115 = 0;
      v116 = 7;
      LOWORD(Src[0]) = 0;
    }
    v30 = *(_QWORD *)(a1 + 1352);
    if ( v30 != 0 && *(_DWORD *)(v30 + 8) != 0 )
      v31 = *(_QWORD *)(a1 + 1360);
    else
      v31 = 0;
    if ( (unsigned int)sub_145AD5C20(qword_14E683C80, v31) == -1 )
    {
      v9 = *(unsigned int *)(a1 + 1732);
      goto LABEL_6;
    }
    v32 = *(_DWORD *)sub_14503CE30(a1);
    LODWORD(v103) = v32;
    v33 = *(_QWORD *)(a1 + 1352);
    if ( v33 != 0 && *(_DWORD *)(v33 + 8) != 0 )
      v34 = *(_QWORD *)(a1 + 1360);
    else
      v34 = 0;
    result = sub_145AD5C20(qword_14E683C80, v34);
    v35 = result;
    if ( v32 >= 0 && (int)result >= 0 )
    {
      v36 = sub_145AD8D10(qword_14E683C80, (unsigned int)v32);
      v37 = _RTDynamicCast(v36, 0, &off_14DCB4FF0, &off_14DCB71E8, 0);
      v38 = sub_145AD8D10(qword_14E683C80, v35);
      v39 = _RTDynamicCast(v38, 0, &off_14DCB4FF0, &off_14DCB64C8, 0);
      v40 = v39;
      if ( v37 != 0
        && v39 != 0
        && (v41 = *(__int16 *)((*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v37 + 1032LL))(v37) + 3408)) == *(_DWORD *)((*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v40 + 1008LL))(v40) + 6652) )
      {
        v104[0] = &v100;
        if ( byte_14DC67898 != 0 )
        {
          LODWORD(v98) = 1808;
          v42 = (_WORD *)sub_146E8C7D0(&unk_14A763860);
          v117[0] = 0;
          v117[2] = 0;
          v117[3] = 7;
          v43 = -1;
          do
            ++v43;
          while ( v42[v43] != 0 );
          sub_14014C8D0(v117, v42);
          __eh34_enter_wind_state(-1, 19);
          v44 = sub_140438530(v135, v117, &v98);
          __eh34_enter_wind_state(19, 20);
          v45 = 3;
          __eh34_enter_wind_state(20, 21);
          __eh34_enter_wind_state(21, 22);
        }
        else
        {
          LODWORD(v98) = 0;
          Src[0] = nullptr;
          v115 = 0;
          v116 = 7;
          sub_14014C8D0(Src, (void *)&Source);
          __eh34_enter_wind_state(-1, 19);
          __eh34_enter_wind_state(19, 20);
          __eh34_enter_wind_state(20, 21);
          v44 = sub_140438530(v132, Src, &v98);
          __eh34_enter_wind_state(21, 22);
          v45 = 12;
        }
        v97 = v45;
        *(_QWORD *)&v100 = 0;
        v101 = 0u;
        v100 = *(_OWORD *)v44;
        v101 = *(_OWORD *)(v44 + 16);
        *(_QWORD *)(v44 + 16) = 0;
        *(_QWORD *)(v44 + 24) = 7;
        *(_WORD *)v44 = 0;
        __wind
        {
          v102 = *(_DWORD *)(v44 + 32);
        }
        __unwind
        {
          unknown_libname_4(v104[0]);
        }
        sub_145ACB980(qword_14E683C80, 12, &v100);
        if ( __eh34_unwind(22) )
        {
          if ( (v45 & 8) != 0 )
            sub_1401512E0(v132);
          __eh34_continue_unwinding(22, 21);
        }
        __eh34_exit_wind_state(22, 21);
        if ( (v45 & 8) != 0 )
        {
          v45 &= ~8u;
          v97 = v45;
          if ( si128.m128i_i64[1] >= 8uLL )
          {
            v46 = 2 * si128.m128i_i64[1] + 2;
            v47 = v132[0];
            if ( v46 >= 0x1000 )
            {
              v46 = 2 * si128.m128i_i64[1] + 41;
              v47 = *(_QWORD *)(v132[0] - 8LL);
              if ( (unsigned __int64)(v132[0] - v47 - 8) > 0x1F )
                invalid_parameter_noinfo_noreturn();
            }
            j_j_scalable_free(v47, v46);
          }
          si128 = _mm_load_si128((const __m128i *)&xmmword_1491AB7C0);
          LOWORD(v132[0]) = 0;
        }
        if ( __eh34_unwind(21) )
        {
          if ( (v97 & 4) != 0 )
            unknown_libname_4(Src);
          __eh34_continue_unwinding(21, 20);
        }
        __eh34_exit_wind_state(21, 20);
        if ( (v45 & 4) != 0 )
        {
          v45 &= ~4u;
          v97 = v45;
          if ( v116 >= 8 )
          {
            v48 = 2 * v116 + 2;
            v49 = Src[0];
            if ( v48 >= 0x1000 )
            {
              v48 = 2 * v116 + 41;
              v49 = *((void **)Src[0] - 1);
              if ( (unsigned __int64)((char *)Src[0] - (char *)v49 - 8) > 0x1F )
                invalid_parameter_noinfo_noreturn();
            }
            j_j_scalable_free(v49, v48);
          }
          v115 = 0;
          v116 = 7;
          LOWORD(Src[0]) = 0;
        }
        if ( __eh34_unwind(20) )
        {
          if ( (v97 & 2) != 0 )
            sub_1401512E0(v135);
          __eh34_continue_unwinding(20, 19);
        }
        __eh34_exit_wind_state(20, 19);
        if ( (v45 & 2) != 0 )
        {
          v45 &= ~2u;
          v97 = v45;
          if ( v136.m128i_i64[1] >= 8uLL )
          {
            v50 = 2 * v136.m128i_i64[1] + 2;
            v51 = v135[0];
            if ( v50 >= 0x1000 )
            {
              v50 = 2 * v136.m128i_i64[1] + 41;
              v51 = *(_QWORD *)(v135[0] - 8LL);
              if ( (unsigned __int64)(v135[0] - v51 - 8) > 0x1F )
                invalid_parameter_noinfo_noreturn();
            }
            j_j_scalable_free(v51, v50);
          }
          v136 = _mm_load_si128((const __m128i *)&xmmword_1491AB7C0);
          LOWORD(v135[0]) = 0;
        }
        if ( __eh34_unwind(19) )
        {
          if ( (v97 & 1) != 0 )
            unknown_libname_4(v117);
          __eh34_propagate_exception_into_caller(19, -1);
        }
        __eh34_exit_wind_state(19, -1);
        if ( (v45 & 1) != 0 )
        {
          if ( v117[3] >= 8u )
          {
            v52 = 2LL * v117[3] + 2;
            v53 = v117[0];
            if ( v52 >= 0x1000 )
            {
              v52 = 2LL * v117[3] + 41;
              v53 = *(_QWORD *)(v117[0] - 8LL);
              if ( (unsigned __int64)(v117[0] - v53 - 8) > 0x1F )
                invalid_parameter_noinfo_noreturn();
            }
            j_j_scalable_free(v53, v52);
          }
          v117[2] = 0;
          v117[3] = 7;
          LOWORD(v117[0]) = 0;
        }
        v54 = sub_14503CDD0(a1);
        v55 = *(_QWORD *)(a1 + 1352);
        if ( v55 != 0 && *(_DWORD *)(v55 + 8) != 0 )
          v56 = *(_QWORD *)(a1 + 1360);
        else
          v56 = 0;
        v57 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v56 + 152LL))(v56);
        v58 = sub_140256DC0(v57 + 16);
        v59 = *(_QWORD *)(a1 + 1352);
        if ( v59 != 0 && *(_DWORD *)(v59 + 8) != 0 )
          v60 = *(_QWORD *)(a1 + 1360);
        else
          v60 = 0;
        v61 = (_DWORD *)(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v60 + 1048LL))(v60);
        sub_146E920A0(v61, &v98);
        v62 = (unsigned __int8)v98;
        v63 = (_DWORD)v98 + *v61 + 196;
        v64 = v61[1];
        if ( v64 != 0 && v63 != 0 && v64 != v63 && retaddr != nullptr )
        {
          sub_146D89B40(retaddr, v61);
          v62 = (unsigned __int8)v98;
        }
        v65 = sub_14503CDD0(a1);
        result = sub_145ABB480(v65);
        if ( result != 0 )
        {
          v66 = *(_DWORD **)(a1 + 1352);
          if ( v66 != nullptr && v66[2] != 0 )
          {
            v66 = *(_DWORD **)(a1 + 1360);
            if ( v66 != nullptr
              && *(_BYTE *)(result + 6440) == 1
              && *(_BYTE *)((*(__int64 (__fastcall **)(_DWORD *))(*(_QWORD *)v66 + 1080LL))(v66) + 285) != 0 )
            {
              *(_DWORD *)((char *)&v117[1] + 6) = -1;
              *(_DWORD *)((char *)&v117[2] + 5) = -1;
              BYTE5(v117[1]) = 1;
              BYTE2(v117[2]) = 0;
              *(_WORD *)((char *)&v117[2] + 3) = v35;
              v67 = qword_14E6387A8;
              if ( qword_14E6387A8 == nullptr )
              {
                v68 = (void *)sub_146E8BA20(176);
                v104[0] = v68;
                __wind
                {
                  if ( v68 != nullptr )
                    v69 = (_LocaleUpdate *)sub_140B896A0(v68);
                  else
                    v69 = nullptr;
                }
                __unwind
                {
                  j_j_scalable_free(v104[0], 176);
                }
                qword_14E6387A8 = v69;
                (**(void (__fastcall ***)(_LocaleUpdate *))v69)(v69);
                v67 = qword_14E6387A8;
              }
              sub_140B8AD40((__int64)v67, (__int64)v117);
            }
          }
          v70 = sub_146D74000(v66);
          sub_146D746E0(v70, 271);
          v72 = sub_146D74000(v71);
          sub_146D76180(v72, (unsigned __int16)v103);
          v74 = sub_146D74000(v73);
          sub_146D75CE0(v74, v54);
          v76 = sub_146D74000(v75);
          sub_146D76180(v76, (unsigned __int16)v35);
          v78 = sub_146D74000(v77);
          sub_146D75CE0(v78, v58);
          v80 = sub_146D74000(v79);
          sub_146D75CC0(v80, v62);
          v82 = sub_146D74000(v81);
          sub_146D76180(v82, 0xFFFF);
          v84 = sub_146D74000(v83);
          sub_146D75CE0(v84, 0xFFFF);
          v99 = 0;
          for ( i = *(_QWORD *)(a1 + 1368); i != *(_QWORD *)(a1 + 1376); i += 32 )
          {
            if ( *(int *)(i + 24) > 0 )
            {
              v86 = *(int *)(i + 24);
              if ( (__int64)(*(_QWORD *)(i + 8) - *(_QWORD *)i) >> 3 > v86 )
              {
                v99 = *(_DWORD *)(*(_QWORD *)i + 8 * v86);
                break;
              }
            }
          }
          v87 = sub_146D74000(i);
          sub_146D75B10(v87, &v99, 4);
          sub_146D75AF0(v88);
          v89 = _RTDynamicCast(*(_QWORD *)(a1 + 1432), 0, &off_14DCB4FF0, &off_14DCB64C8, 0);
          v90 = sub_14503D130();
          v91 = *(_QWORD *)(a1 + 1352);
          if ( v91 != 0 && *(_DWORD *)(v91 + 8) != 0 )
            v4 = *(_QWORD *)(a1 + 1360);
          sub_144CCD4B0(v90, v89, v4, v35, v35);
          v92 = sub_14503D130();
          if ( (unsigned __int8)sub_144CCCEE0(v92, v58) == 1 )
          {
            v93 = sub_14503D130();
            sub_140697090(v93, v35);
            v94 = sub_14503D130();
            sub_1410AF2D0(v94, v58);
          }
          return (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 264LL))(a1);
        }
      }
      else
      {
        v95 = *(_QWORD *)(a1 + 848);
        v96 = sub_14723C170(35355);
        sub_14668C520(v95, 2875, v96, 0);
        sub_146694510(qword_14E683C78, 1321, -1, 0, 1);
        return sub_1466963D0(*(_QWORD *)&qword_14E683C78, 0);
      }
    }
  }
  return result;
}
