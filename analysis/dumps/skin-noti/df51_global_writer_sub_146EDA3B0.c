// global_writer_sub_146EDA3B0

__int64 __fastcall sub_146EDA3B0(__int64 a1)
{
  __int64 result; // rax
  __int64 v3; // rcx
  __int64 v4; // rbx
  float v5; // xmm2_4
  float v6; // xmm1_4
  int v7; // eax
  _QWORD *v8; // rdx
  int v9; // esi
  __int64 v10; // rax
  __int64 v11; // r8
  volatile signed __int32 *v12; // rbx
  __int64 v13; // rdi
  __int64 v14; // rcx
  __int64 v15; // rcx
  int v16; // ebx
  int v17; // eax
  int v18; // eax
  __int64 v19; // rcx
  int v20; // r9d
  int v21; // ebx
  int v22; // eax
  int v23; // eax
  char v24; // al
  volatile signed __int32 *v25; // rbx
  __int64 v26; // rdi
  int v27; // ecx
  unsigned int v28; // r15d
  int v29; // ebx
  int v30; // eax
  unsigned int v31; // edi
  unsigned __int8 v32; // r12
  int v33; // ebx
  int v34; // eax
  unsigned __int8 v35; // r14
  int v36; // ebx
  int v37; // eax
  int v38; // ebx
  int v39; // eax
  int v40; // r8d
  __int64 v41; // rdx
  int v42; // edi
  __int64 v43; // rax
  __int64 v44; // r8
  volatile signed __int32 *v45; // rbx
  __int64 v46; // rsi
  __int64 v47; // rcx
  __int64 v48; // rcx
  int v49; // ebx
  int v50; // eax
  int v51; // eax
  __int64 v52; // rcx
  int v53; // r9d
  int v54; // r9d
  __int64 v55; // rcx
  __int64 v56; // r8
  char v57; // al
  volatile signed __int32 *v58; // rbx
  __int64 v59; // rdi
  int v60; // eax
  _QWORD *v61; // rdx
  int v62; // ecx
  unsigned int v63; // r15d
  int v64; // ebx
  int v65; // eax
  unsigned int v66; // edi
  unsigned __int8 v67; // r12
  int v68; // ebx
  int v69; // eax
  unsigned __int8 v70; // r14
  int v71; // ebx
  int v72; // eax
  int v73; // ebx
  int v74; // eax
  int v75; // edi
  __int64 v76; // rax
  __int64 v77; // r8
  volatile signed __int32 *v78; // rbx
  __int64 v79; // rsi
  __int64 v80; // rcx
  __int64 v81; // rcx
  char v82; // al
  __int64 v83; // rcx
  __int64 v84; // r8
  char v85; // al
  volatile signed __int32 *v86; // rbx
  __int64 v87; // rsi
  unsigned int v88; // esi
  __int64 v89; // rax
  __int64 v90; // r8
  volatile signed __int32 *v91; // rbx
  __int64 v92; // rdi
  volatile signed __int32 *v93; // rbx
  __int64 v94; // rdi
  __int64 v95; // rcx
  char v96; // al
  volatile signed __int32 *v97; // rbx
  __int64 v98; // rdi
  float v99; // xmm2_4
  int v100; // eax
  _QWORD *v101; // rdx
  int v102; // [rsp+70h] [rbp-90h] BYREF
  int v103; // [rsp+74h] [rbp-8Ch]
  int v104; // [rsp+78h] [rbp-88h] BYREF
  int v105; // [rsp+7Ch] [rbp-84h]
  int v106; // [rsp+80h] [rbp-80h] BYREF
  int v107; // [rsp+84h] [rbp-7Ch]
  _BYTE v108[16]; // [rsp+88h] [rbp-78h] BYREF
  _DWORD v109[2]; // [rsp+98h] [rbp-68h] BYREF
  _DWORD v110[2]; // [rsp+A0h] [rbp-60h] BYREF
  _DWORD v111[2]; // [rsp+A8h] [rbp-58h] BYREF
  _DWORD v112[2]; // [rsp+B0h] [rbp-50h] BYREF
  _DWORD v113[2]; // [rsp+B8h] [rbp-48h] BYREF
  __int64 v114; // [rsp+C0h] [rbp-40h] BYREF
  _DWORD v115[2]; // [rsp+C8h] [rbp-38h] BYREF
  _DWORD v116[2]; // [rsp+D0h] [rbp-30h] BYREF
  _DWORD v117[2]; // [rsp+D8h] [rbp-28h] BYREF
  _DWORD v118[2]; // [rsp+E0h] [rbp-20h] BYREF
  _DWORD v119[2]; // [rsp+E8h] [rbp-18h] BYREF
  _DWORD v120[2]; // [rsp+F0h] [rbp-10h] BYREF
  __int64 v121; // [rsp+F8h] [rbp-8h] BYREF
  _DWORD v122[2]; // [rsp+100h] [rbp+0h] BYREF
  _DWORD v123[2]; // [rsp+108h] [rbp+8h] BYREF
  _DWORD v124[2]; // [rsp+110h] [rbp+10h] BYREF
  _DWORD v125[2]; // [rsp+118h] [rbp+18h] BYREF
  __int64 v126; // [rsp+120h] [rbp+20h] BYREF
  _DWORD v127[2]; // [rsp+128h] [rbp+28h] BYREF
  _DWORD v128[2]; // [rsp+130h] [rbp+30h] BYREF
  __int64 v129; // [rsp+138h] [rbp+38h] BYREF
  _DWORD v130[2]; // [rsp+140h] [rbp+40h] BYREF
  _DWORD v131[2]; // [rsp+148h] [rbp+48h] BYREF
  _DWORD v132[2]; // [rsp+150h] [rbp+50h] BYREF
  _DWORD v133[2]; // [rsp+158h] [rbp+58h] BYREF
  __int64 v134; // [rsp+160h] [rbp+60h] BYREF
  _DWORD v135[2]; // [rsp+168h] [rbp+68h] BYREF
  _DWORD v136[2]; // [rsp+170h] [rbp+70h] BYREF
  __int64 v137; // [rsp+178h] [rbp+78h] BYREF
  _DWORD v138[2]; // [rsp+180h] [rbp+80h] BYREF
  __int128 v139; // [rsp+188h] [rbp+88h] BYREF
  __int128 v140; // [rsp+198h] [rbp+98h] BYREF
  __int128 v141; // [rsp+1A8h] [rbp+A8h] BYREF
  __int128 v142; // [rsp+1B8h] [rbp+B8h] BYREF
  __int128 v143; // [rsp+1C8h] [rbp+C8h] BYREF
  __int128 v144; // [rsp+1D8h] [rbp+D8h] BYREF
  __int64 v145; // [rsp+1E8h] [rbp+E8h]
  volatile signed __int32 *v146; // [rsp+1F0h] [rbp+F0h]
  __int64 v147; // [rsp+1F8h] [rbp+F8h]
  unsigned int v148; // [rsp+248h] [rbp+148h] BYREF
  int v149; // [rsp+24Ch] [rbp+14Ch]
  int v150; // [rsp+250h] [rbp+150h]
  int v151; // [rsp+258h] [rbp+158h] BYREF
  int v152; // [rsp+25Ch] [rbp+15Ch]

  v147 = -2;
  result = sub_141FB6530(a1);
  if ( !(_BYTE)result )
    return result;
  sub_146EC9ED0(a1, &v104);
  sub_146EA9480(a1 + 1632);
  if ( *(_BYTE *)(a1 + 1549) )
  {
    if ( !(unsigned __int8)sub_146ED0010(a1)
      && !(unsigned __int8)sub_146ECFF30(a1)
      && !(unsigned __int8)sub_146ED4C80(a1) )
    {
      if ( (unsigned __int8)sub_146ED0010(a1) )
        goto LABEL_12;
      sub_146E9FBC0(a1 + 1584);
      if ( (*(_DWORD *)(a1 + 1556) & 2) != 0 )
        goto LABEL_12;
      v3 = a1 + 1552;
      goto LABEL_11;
    }
    *(_BYTE *)(a1 + 1547) = 1;
    sub_146E9FBC0(a1 + 1552);
    if ( (*(_DWORD *)(a1 + 1588) & 2) == 0 )
    {
      v3 = a1 + 1584;
LABEL_11:
      sub_146E9FBD0(v3, 0, 0);
    }
  }
LABEL_12:
  if ( *(_BYTE *)(a1 + 1288) )
  {
    if ( (unsigned __int8)sub_146ECFF40(a1) )
    {
      v4 = 2;
    }
    else if ( (unsigned __int8)sub_146ED0010(a1) || (v4 = 0, (unsigned __int8)sub_146ECFF30(a1)) )
    {
      v4 = 1;
    }
    *(_DWORD *)(a1 + 1292) = *(_DWORD *)(a1 + 4 * v4 + 1296);
    v5 = (float)(*(float *)(a1 + 4 * v4 + 1296) * *(float *)(a1 + 156)) * -0.5;
    *(float *)(a1 + 8 * v4 + 1132) = (float)(*(float *)(a1 + 4 * v4 + 1296) * *(float *)(a1 + 152)) * -0.5;
    *(float *)(a1 + 8 * v4 + 1136) = v5;
    v6 = *(float *)(a1 + 1292);
    *(float *)(a1 + 1104) = v6 + *(float *)(a1 + 1104);
    *(float *)(a1 + 1108) = v6 + *(float *)(a1 + 1108);
  }
  if ( (unsigned __int8)sub_146ECA4A0(a1) )
  {
    if ( ((unsigned __int8)sub_146ECFD90(a1) || *(_BYTE *)(a1 + 1625) && *(_BYTE *)(a1 + 1624))
      && *(_QWORD *)(a1 + 968)
      && !*(_BYTE *)(a1 + 1940) )
    {
      v7 = sub_146E74CB0();
      v8 = (_QWORD *)(a1 + 952);
      if ( *(_QWORD *)(a1 + 976) >= 8u )
        v8 = (_QWORD *)*v8;
      sub_146E76C50(v7, (_DWORD)v8, 0, 0, 0, 0, 0xFFFF, -1, 0, 0, -1, -1, 0, 0);
    }
    if ( (unsigned __int8)sub_146ECFF40(a1) || *(_BYTE *)(a1 + 1625) )
    {
      v75 = *(_DWORD *)(a1 + 1124);
      v76 = sub_146EE1F80(a1, v108, 2);
      sub_146EDD490(a1, &v148, 2, v76);
      sub_146EE3540(a1, 2);
      v78 = *(volatile signed __int32 **)(a1 + 1984);
      if ( v78 )
      {
        _InterlockedIncrement(v78 + 2);
        v78 = *(volatile signed __int32 **)(a1 + 1984);
      }
      v79 = *(_QWORD *)(a1 + 1976);
      if ( v78 )
      {
        if ( _InterlockedExchangeAdd(v78 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v78)(v78);
          if ( _InterlockedExchangeAdd(v78 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v78 + 8LL))(v78);
        }
      }
      if ( v79 )
      {
        v143 = 0;
        v80 = *(_QWORD *)(a1 + 1984);
        if ( v80 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v80 + 8));
          v80 = *(_QWORD *)(a1 + 1984);
        }
        *(_QWORD *)&v143 = *(_QWORD *)(a1 + 1976);
        *((_QWORD *)&v143 + 1) = v80;
        sub_146EAA830(&v143);
      }
      v81 = *(unsigned __int8 *)(a1 + 1100);
      if ( (_BYTE)v81 && (_BYTE)v81 != 14 )
      {
        v126 = 0;
        LOBYTE(v77) = 3;
        sub_146ED7180(v81, 0, v77, &v126);
      }
      v82 = *(_BYTE *)(a1 + 1321);
      if ( *(_BYTE *)(a1 + 1101) )
      {
        if ( v82 )
        {
          v127[0] = v148;
          v127[1] = v149;
          sub_146EE23D0(a1, 0, v127);
        }
        else
        {
          v128[0] = v148;
          v128[1] = v149;
          sub_146EE22C0(a1, (unsigned int)v128, 0, v75, 2);
        }
        v129 = 0;
        LOBYTE(v84) = 3;
        LOBYTE(v83) = 2;
        sub_146ED7180(v83, 0, v84, &v129);
        if ( *(_BYTE *)(a1 + 1321) )
        {
          v130[0] = v148;
          v130[1] = v149;
          sub_146EE23D0(a1, 2, v130);
        }
        else
        {
          v131[0] = v148;
          v131[1] = v149;
          sub_146EE22C0(a1, (unsigned int)v131, 2, v75, 4);
        }
        sub_146ED70A0(0);
      }
      else if ( v82 )
      {
        v132[0] = v148;
        v132[1] = v149;
        sub_146EE23D0(a1, 2, v132);
      }
      else
      {
        v133[0] = v148;
        v133[1] = v149;
        sub_146EE22C0(a1, (unsigned int)v133, 2, v75, 4);
      }
      v85 = *(_BYTE *)(a1 + 1100);
      if ( v85 && v85 != 14 )
        sub_146ED70A0(0);
      v86 = *(volatile signed __int32 **)(a1 + 1984);
      if ( v86 )
      {
        _InterlockedIncrement(v86 + 2);
        v86 = *(volatile signed __int32 **)(a1 + 1984);
      }
      v87 = *(_QWORD *)(a1 + 1976);
      if ( v86 )
      {
        if ( _InterlockedExchangeAdd(v86 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v86)(v86);
          if ( _InterlockedExchangeAdd(v86 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v86 + 8LL))(v86);
        }
      }
      if ( v87 )
        sub_146EAA6E0();
      if ( !*(_BYTE *)(a1 + 1017) )
        *(_BYTE *)(a1 + 1017) = 1;
      if ( *(_QWORD *)(a1 + 1024) )
      {
        v104 = *(_DWORD *)(a1 + 1088);
        v105 = *(_DWORD *)(a1 + 1092);
        sub_146EDBDA0(
          a1,
          *(_DWORD *)(a1 + 1072) & 0xFFFFFF
        | ((unsigned __int8)(int)(float)((float)(v75 * *(unsigned __int8 *)(a1 + 1075)) * 0.0039215689) << 24),
          (unsigned int)&v104,
          2,
          0);
      }
      sub_146EE1880(a1, 2u, 255);
      v41 = 2;
    }
    else if ( (unsigned __int8)sub_146ED0010(a1)
           || (unsigned __int8)sub_146ECFF30(a1)
           || (unsigned __int8)sub_146ED4C80(a1) )
    {
      v42 = *(_DWORD *)(a1 + 1120);
      v150 = v42;
      v43 = sub_146EE1F80(a1, v108, 1);
      sub_146EDD490(a1, &v151, 1, v43);
      sub_146EE3540(a1, 1);
      v45 = *(volatile signed __int32 **)(a1 + 1968);
      if ( v45 )
      {
        _InterlockedIncrement(v45 + 2);
        v45 = *(volatile signed __int32 **)(a1 + 1968);
      }
      v46 = *(_QWORD *)(a1 + 1960);
      if ( v45 )
      {
        if ( _InterlockedExchangeAdd(v45 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v45)(v45);
          if ( _InterlockedExchangeAdd(v45 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v45 + 8LL))(v45);
        }
      }
      if ( v46 )
      {
        v141 = 0;
        v47 = *(_QWORD *)(a1 + 1968);
        if ( v47 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v47 + 8));
          v47 = *(_QWORD *)(a1 + 1968);
        }
        *(_QWORD *)&v141 = *(_QWORD *)(a1 + 1960);
        *((_QWORD *)&v141 + 1) = v47;
        sub_146EAA830(&v141);
      }
      v48 = *(unsigned __int8 *)(a1 + 1100);
      if ( (_BYTE)v48 && (_BYTE)v48 != 14 )
      {
        v114 = 0;
        LOBYTE(v44) = 3;
        sub_146ED7180(v48, 0, v44, &v114);
      }
      if ( *(_BYTE *)(a1 + 1549) )
      {
        v49 = dword_14DC6B498;
        v50 = sub_146E9F840(a1 + 1584);
        v51 = sub_146EA0D40(0, *(_DWORD *)(a1 + 1120), v50, v49, 5);
        v42 = v51;
        v150 = v51;
        *(_DWORD *)(a1 + 1620) = v51;
        if ( *(_QWORD *)(a1 + 2016) )
        {
          *(float *)(a1 + 2012) = (float)v51 * 0.0039215689;
          sub_146EE3670(a1);
          v142 = 0;
          v52 = *(_QWORD *)(a1 + 2024);
          if ( v52 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v52 + 8));
            v52 = *(_QWORD *)(a1 + 2024);
          }
          *(_QWORD *)&v142 = *(_QWORD *)(a1 + 2016);
          *((_QWORD *)&v142 + 1) = v52;
          sub_146EAA830(&v142);
          v53 = *(_DWORD *)(a1 + 1116);
          if ( *(_BYTE *)(a1 + 1321) )
          {
            v115[0] = v151;
            v115[1] = v152;
            sub_146EE23D0(a1, 0, v115);
          }
          else
          {
            v116[0] = v151;
            v116[1] = v152;
            sub_146EE22C0(a1, (unsigned int)v116, 0, v53, 4);
          }
          sub_146EAA6E0();
        }
        else
        {
          v54 = *(_DWORD *)(a1 + 1116);
          if ( *(_BYTE *)(a1 + 1321) )
          {
            v117[0] = v151;
            v117[1] = v152;
            sub_146EE23D0(a1, 0, v117);
          }
          else
          {
            v118[0] = v151;
            v118[1] = v152;
            sub_146EE22C0(a1, (unsigned int)v118, 0, v54, 4);
          }
        }
      }
      if ( *(_BYTE *)(a1 + 1101) )
      {
        if ( *(_BYTE *)(a1 + 1321) )
        {
          v119[0] = v151;
          v119[1] = v152;
          sub_146EE23D0(a1, 0, v119);
        }
        else
        {
          v120[0] = v151;
          v120[1] = v152;
          sub_146EE22C0(a1, (unsigned int)v120, 0, v42, 4);
        }
        v121 = 0;
        LOBYTE(v56) = 3;
        LOBYTE(v55) = 2;
        sub_146ED7180(v55, 0, v56, &v121);
        if ( *(_BYTE *)(a1 + 1321) )
        {
          v122[0] = v151;
          v122[1] = v152;
          sub_146EE23D0(a1, 1, v122);
        }
        else
        {
          v123[0] = v151;
          v123[1] = v152;
          sub_146EE22C0(a1, (unsigned int)v123, 1, v42, 4);
        }
        sub_146ED70A0(0);
      }
      else if ( !*(_BYTE *)(a1 + 1549) || !*(_QWORD *)(a1 + 2016) )
      {
        if ( *(_BYTE *)(a1 + 1321) )
        {
          v124[0] = v151;
          v124[1] = v152;
          sub_146EE23D0(a1, 1, v124);
        }
        else
        {
          v125[0] = v151;
          v125[1] = v152;
          sub_146EE22C0(a1, (unsigned int)v125, 1, v42, 4);
        }
      }
      v57 = *(_BYTE *)(a1 + 1100);
      if ( v57 && v57 != 14 )
        sub_146ED70A0(0);
      v58 = *(volatile signed __int32 **)(a1 + 1984);
      if ( v58 )
      {
        _InterlockedIncrement(v58 + 2);
        v58 = *(volatile signed __int32 **)(a1 + 1984);
      }
      v59 = *(_QWORD *)(a1 + 1976);
      if ( v58 )
      {
        if ( _InterlockedExchangeAdd(v58 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v58)(v58);
          if ( _InterlockedExchangeAdd(v58 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v58 + 8LL))(v58);
        }
      }
      if ( v59 )
        sub_146EAA6E0();
      if ( !*(_BYTE *)(a1 + 1016) )
      {
        if ( *(_QWORD *)(a1 + 1000) )
        {
          v60 = sub_146E74CB0();
          v61 = (_QWORD *)(a1 + 984);
          if ( *(_QWORD *)(a1 + 1008) >= 8u )
            v61 = (_QWORD *)*v61;
          sub_146E76C50(v60, (_DWORD)v61, 0, 0, 0, 0, 0xFFFF, -1, 0, 0, -1, -1, 0, 0);
        }
        *(_BYTE *)(a1 + 1016) = 1;
      }
      v62 = *(_DWORD *)(a1 + 1068);
      v148 = v62;
      if ( *(_BYTE *)(a1 + 1549) )
      {
        v63 = *(_DWORD *)(a1 + 1064);
        v64 = dword_14DC6B498;
        v65 = sub_146E9F840(a1 + 1584);
        v66 = v148;
        v67 = sub_146EA0D40((unsigned __int8)v63, (unsigned __int8)v148, v65, v64, 5);
        v68 = dword_14DC6B498;
        v69 = sub_146E9F840(a1 + 1584);
        v70 = sub_146EA0D40(BYTE1(v63), BYTE1(v66), v69, v68, 5);
        v71 = dword_14DC6B498;
        v72 = sub_146E9F840(a1 + 1584);
        LOBYTE(v66) = sub_146EA0D40(BYTE2(v63), BYTE2(v66), v72, v71, 5);
        v73 = dword_14DC6B498;
        v74 = sub_146E9F840(a1 + 1584);
        v62 = v67
            | (v70 << 8)
            | (((unsigned __int8)v66
              | ((unsigned __int8)(int)(float)((float)(int)(*(_DWORD *)(a1 + 1120)
                                                          * sub_146EA0D40(HIBYTE(v63), HIBYTE(v148), v74, v73, 5))
                                             * 0.0039215689) << 8)) << 16);
      }
      if ( *(_QWORD *)(a1 + 1024) )
      {
        v104 = *(_DWORD *)(a1 + 1088);
        v105 = *(_DWORD *)(a1 + 1092);
        sub_146EDBDA0(a1, v62, (unsigned int)&v104, 1, *(_BYTE *)(a1 + 1549));
      }
      if ( *(_BYTE *)(a1 + 1549) == 1 )
        sub_146EE1880(a1, 0, 255);
      sub_146EE1880(a1, 1u, v150);
      v41 = 1;
    }
    else
    {
      v9 = *(_DWORD *)(a1 + 1116);
      v10 = sub_146EE1F80(a1, v108, 0);
      sub_146EDD490(a1, &v102, 0, v10);
      sub_146EE3540(a1, 0);
      v12 = *(volatile signed __int32 **)(a1 + 1952);
      if ( v12 )
      {
        _InterlockedIncrement(v12 + 2);
        v12 = *(volatile signed __int32 **)(a1 + 1952);
      }
      v13 = *(_QWORD *)(a1 + 1944);
      if ( v12 )
      {
        if ( _InterlockedExchangeAdd(v12 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v12)(v12);
          if ( _InterlockedExchangeAdd(v12 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v12 + 8LL))(v12);
        }
      }
      if ( v13 )
      {
        v139 = 0;
        v14 = *(_QWORD *)(a1 + 1952);
        if ( v14 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v14 + 8));
          v14 = *(_QWORD *)(a1 + 1952);
        }
        *(_QWORD *)&v139 = *(_QWORD *)(a1 + 1944);
        *((_QWORD *)&v139 + 1) = v14;
        sub_146EAA830(&v139);
      }
      v15 = *(unsigned __int8 *)(a1 + 1100);
      if ( (_BYTE)v15 && (_BYTE)v15 != 14 )
      {
        v137 = 0;
        LOBYTE(v11) = 3;
        sub_146ED7180(v15, 0, v11, &v137);
      }
      if ( *(_BYTE *)(a1 + 1549) && *(_QWORD *)(a1 + 2016) )
      {
        v16 = dword_14DC6B49C;
        v17 = sub_146E9F840(a1 + 1552);
        v18 = sub_146EA0D40(*(_DWORD *)(a1 + 1120), 0, v17, v16, 5);
        *(_DWORD *)(a1 + 1616) = v18;
        *(float *)(a1 + 2012) = (float)v18 * 0.0039215689;
        if ( !*(_BYTE *)(a1 + 1547) )
          *(_DWORD *)(a1 + 2012) = 0;
        sub_146EE3670(a1);
        v140 = 0;
        v19 = *(_QWORD *)(a1 + 2024);
        if ( v19 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v19 + 8));
          v19 = *(_QWORD *)(a1 + 2024);
        }
        *(_QWORD *)&v140 = *(_QWORD *)(a1 + 2016);
        *((_QWORD *)&v140 + 1) = v19;
        sub_146EAA830(&v140);
        v20 = *(_DWORD *)(a1 + 1116);
        if ( *(_BYTE *)(a1 + 1321) )
        {
          v138[0] = v102;
          v138[1] = v103;
          sub_146EE23D0(a1, 0, v138);
        }
        else
        {
          v109[0] = v102;
          v109[1] = v103;
          sub_146EE22C0(a1, (unsigned int)v109, 0, v20, 4);
        }
        sub_146EAA6E0();
      }
      else if ( *(_BYTE *)(a1 + 1321) )
      {
        v110[0] = v102;
        v110[1] = v103;
        sub_146EE23D0(a1, 0, v110);
      }
      else
      {
        v111[0] = v102;
        v111[1] = v103;
        sub_146EE22C0(a1, (unsigned int)v111, 0, v9, 4);
      }
      if ( *(_BYTE *)(a1 + 1549) )
      {
        if ( !*(_QWORD *)(a1 + 2016) )
        {
          v21 = dword_14DC6B49C;
          v22 = sub_146E9F840(a1 + 1552);
          v23 = sub_146EA0D40(*(_DWORD *)(a1 + 1120), 0, v22, v21, 5);
          *(_DWORD *)(a1 + 1616) = v23;
          if ( v23 > 0 )
          {
            if ( *(_BYTE *)(a1 + 1547) )
            {
              if ( *(_BYTE *)(a1 + 1321) )
              {
                v112[0] = v102;
                v112[1] = v103;
                sub_146EE23D0(a1, 1, v112);
              }
              else
              {
                v113[0] = v102;
                v113[1] = v103;
                sub_146EE22C0(a1, (unsigned int)v113, 1, v23, 4);
              }
            }
          }
        }
      }
      v24 = *(_BYTE *)(a1 + 1100);
      if ( v24 && v24 != 14 )
        sub_146ED70A0(0);
      v25 = *(volatile signed __int32 **)(a1 + 1984);
      if ( v25 )
      {
        _InterlockedIncrement(v25 + 2);
        v25 = *(volatile signed __int32 **)(a1 + 1984);
      }
      v26 = *(_QWORD *)(a1 + 1976);
      if ( v25 )
      {
        if ( _InterlockedExchangeAdd(v25 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v25)(v25);
          if ( _InterlockedExchangeAdd(v25 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v25 + 8LL))(v25);
        }
      }
      if ( v26 )
        sub_146EAA6E0();
      *(_WORD *)(a1 + 1016) = 0;
      v27 = *(_DWORD *)(a1 + 1064);
      v148 = v27;
      if ( *(_BYTE *)(a1 + 1549) )
      {
        v28 = *(_DWORD *)(a1 + 1068);
        v29 = dword_14DC6B498;
        v30 = sub_146E9F840(a1 + 1552);
        v31 = v148;
        v32 = sub_146EA0D40((unsigned __int8)v28, (unsigned __int8)v148, v30, v29, 5);
        v33 = dword_14DC6B498;
        v34 = sub_146E9F840(a1 + 1552);
        v35 = sub_146EA0D40(BYTE1(v28), BYTE1(v31), v34, v33, 5);
        v36 = dword_14DC6B498;
        v37 = sub_146E9F840(a1 + 1552);
        LOBYTE(v31) = sub_146EA0D40(BYTE2(v28), BYTE2(v31), v37, v36, 5);
        v38 = dword_14DC6B498;
        v39 = sub_146E9F840(a1 + 1552);
        v27 = v32
            | (v35 << 8)
            | (((unsigned __int8)v31
              | ((unsigned __int8)(int)(float)((float)(int)(*(_DWORD *)(a1 + 1116)
                                                          * sub_146EA0D40(HIBYTE(v28), HIBYTE(v148), v39, v38, 5))
                                             * 0.0039215689) << 8)) << 16);
      }
      if ( *(_QWORD *)(a1 + 1024) )
      {
        v104 = *(_DWORD *)(a1 + 1088);
        v105 = *(_DWORD *)(a1 + 1092);
        sub_146EDBDA0(a1, v27, (unsigned int)&v104, 0, *(_BYTE *)(a1 + 1549));
      }
      sub_146EE1880(a1, 0, 255);
      if ( *(_BYTE *)(a1 + 1549) == 1 )
      {
        v40 = *(_DWORD *)(a1 + 1616);
        if ( v40 > 0 )
        {
          if ( *(_BYTE *)(a1 + 1547) )
            sub_146EE1880(a1, 1u, v40);
        }
      }
      v41 = 0;
    }
  }
  else
  {
    v88 = *(_DWORD *)(a1 + 1128);
    v89 = sub_146EE1F80(a1, v108, 3);
    sub_146EDD490(a1, &v106, 3, v89);
    sub_146EE3540(a1, 3);
    v91 = *(volatile signed __int32 **)(a1 + 2000);
    if ( v91 )
    {
      _InterlockedIncrement(v91 + 2);
      v91 = *(volatile signed __int32 **)(a1 + 2000);
    }
    v92 = *(_QWORD *)(a1 + 1992);
    if ( v91 )
    {
      if ( _InterlockedExchangeAdd(v91 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v91)(v91);
        if ( _InterlockedExchangeAdd(v91 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v91 + 8LL))(v91);
      }
    }
    if ( v92 )
    {
      v93 = *(volatile signed __int32 **)(a1 + 2000);
      if ( v93 )
      {
        _InterlockedIncrement(v93 + 2);
        v93 = *(volatile signed __int32 **)(a1 + 2000);
      }
      v94 = *(_QWORD *)(a1 + 1992);
      v145 = v94;
      v146 = v93;
      if ( !(unsigned int)sub_140696260(v94) )
        sub_146FB5A40(v94, v88);
      v144 = 0;
      if ( v93 )
      {
        _InterlockedIncrement(v93 + 2);
        v94 = v145;
      }
      *(_QWORD *)&v144 = v94;
      *((_QWORD *)&v144 + 1) = v93;
      sub_146EAA830(&v144);
      if ( v93 )
      {
        if ( _InterlockedExchangeAdd(v93 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v93)(v93);
          if ( _InterlockedExchangeAdd(v93 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v93 + 8LL))(v93);
        }
      }
    }
    v95 = *(unsigned __int8 *)(a1 + 1100);
    if ( (_BYTE)v95 && (_BYTE)v95 != 14 )
    {
      v134 = 0;
      LOBYTE(v90) = 3;
      sub_146ED7180(v95, 0, v90, &v134);
    }
    if ( *(_QWORD *)(a1 + 880) )
    {
      if ( *(_BYTE *)(a1 + 1321) )
      {
        v135[0] = v106;
        v135[1] = v107;
        sub_146EE23D0(a1, 3, v135);
      }
      else
      {
        v136[0] = v106;
        v136[1] = v107;
        sub_146EE22C0(a1, (unsigned int)v136, 3, v88, 4);
      }
    }
    v96 = *(_BYTE *)(a1 + 1100);
    if ( v96 && v96 != 14 )
      sub_146ED70A0(0);
    v97 = *(volatile signed __int32 **)(a1 + 2000);
    if ( v97 )
    {
      _InterlockedIncrement(v97 + 2);
      v97 = *(volatile signed __int32 **)(a1 + 2000);
    }
    v98 = *(_QWORD *)(a1 + 1992);
    if ( v97 )
    {
      if ( _InterlockedExchangeAdd(v97 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v97)(v97);
        if ( _InterlockedExchangeAdd(v97 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v97 + 8LL))(v97);
      }
    }
    if ( v98 )
      sub_146EAA6E0();
    if ( *(_QWORD *)(a1 + 1024) )
    {
      v104 = *(_DWORD *)(a1 + 1088);
      v105 = *(_DWORD *)(a1 + 1092);
      sub_146EDBDA0(
        a1,
        *(_DWORD *)(a1 + 1076) & 0xFFFFFF
      | ((unsigned __int8)(int)(float)((float)(int)(v88 * *(unsigned __int8 *)(a1 + 1079)) * 0.0039215689) << 24),
        (unsigned int)&v104,
        3,
        1);
    }
    sub_146EE1880(a1, 3u, 255);
    v41 = 3;
  }
  result = (*(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)a1 + 704LL))(a1, v41);
  if ( *(_BYTE *)(a1 + 1288) )
  {
    v99 = *(float *)(a1 + 1292);
    *(float *)(a1 + 1104) = *(float *)(a1 + 1104) - v99;
    *(float *)(a1 + 1108) = *(float *)(a1 + 1108) - v99;
  }
  if ( *(_BYTE *)(a1 + 1940) )
  {
    result = *(unsigned int *)(a1 + 496);
    if ( dword_14DC6B63C == (_DWORD)result || *(_BYTE *)(a1 + 1625) && *(_BYTE *)(a1 + 1624) )
    {
      if ( *(_QWORD *)(a1 + 968) )
      {
        v100 = sub_146E74CB0();
        v101 = (_QWORD *)(a1 + 952);
        if ( *(_QWORD *)(a1 + 976) >= 8u )
          v101 = (_QWORD *)*v101;
        return sub_146E76C50(v100, (_DWORD)v101, 0, 0, 0, 0, 0xFFFF, -1, 0, 0, -1, -1, 0, 0);
      }
    }
  }
  return result;
}

