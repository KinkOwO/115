// reader_sub_1444E2120

_UNKNOWN **__fastcall sub_1444E2120(__int64 a1)
{
  _UNKNOWN **result; // rax
  __int64 v3; // rax
  __int64 v4; // rcx
  _UNKNOWN **v5; // rbx
  __int64 v6; // rax
  int v7; // edi
  __int64 v8; // rcx
  __int64 v9; // rax
  int v10; // esi
  int v11; // ecx
  __int64 v12; // rcx
  __int64 v13; // rdx
  __int64 v14; // rdx
  __int64 v15; // rcx
  __int64 v16; // rcx
  int v17; // edx
  __int64 v18; // rdx
  __int64 v19; // rcx
  __int64 v20; // rdx
  double v21; // xmm0_8
  float v22; // xmm6_4
  double v23; // xmm0_8
  int v24; // ebx
  __int64 v25; // r8
  __m128i v26; // xmm7
  __int64 v27; // rdx
  float v28; // xmm7_4
  double v29; // xmm0_8
  __int64 v30; // rdi
  void (__fastcall *v31)(__int64); // rbx
  __int64 v32; // rcx
  __int64 v33; // rax
  _UNKNOWN **v34; // r8
  int v35; // edx
  _QWORD *v36; // rcx
  _QWORD *v37; // rbx
  _QWORD *v38; // r14
  unsigned int v39; // r13d
  int v40; // edi
  int v41; // r12d
  __int64 v42; // rdx
  __int64 v43; // rcx
  __int64 v44; // rax
  void (__fastcall ***v45)(_QWORD); // rcx
  __int64 v46; // rdx
  char v47; // al
  __int64 v48; // rcx
  __int64 v49; // rdx
  int v50; // edi
  unsigned int v51; // esi
  int v52; // esi
  int v53; // r13d
  __int64 v54; // r8
  __int64 v55; // rdx
  __int64 v56; // rcx
  unsigned int v57; // eax
  int v58; // edi
  __int64 v59; // rcx
  __int64 v60; // rax
  void (__fastcall ***v61)(_QWORD); // rcx
  char v62; // al
  int v63; // edx
  __int64 v64; // rdx
  double v65; // xmm0_8
  float v66; // xmm9_4
  double v67; // xmm0_8
  float v68; // xmm7_4
  double v69; // xmm0_8
  float v70; // xmm7_4
  float v71; // xmm13_4
  float v72; // xmm12_4
  double v73; // xmm0_8
  double v74; // xmm0_8
  __int64 v75; // rdx
  double v76; // xmm0_8
  double v77; // xmm0_8
  __int64 v78; // rsi
  void (__fastcall *v79)(__int64, _QWORD); // rdi
  unsigned __int8 v80; // al
  __int64 v81; // rcx
  __int64 v82; // rax
  __int64 v83; // rcx
  __int64 v84; // rax
  __int64 v85; // rdx
  __int64 v86; // rcx
  _QWORD *v87; // rcx
  _QWORD *v88; // rbx
  __int64 v89; // rdi
  __int64 v90; // rcx
  int v91; // [rsp+28h] [rbp-E0h] BYREF
  float v92; // [rsp+2Ch] [rbp-DCh]
  float v93; // [rsp+30h] [rbp-D8h]
  float v94; // [rsp+34h] [rbp-D4h]
  int v95; // [rsp+38h] [rbp-D0h]
  unsigned int v96; // [rsp+3Ch] [rbp-CCh]
  int v97; // [rsp+40h] [rbp-C8h]
  float v98; // [rsp+44h] [rbp-C4h]
  _UNKNOWN **v99; // [rsp+48h] [rbp-C0h]
  __int64 v100; // [rsp+50h] [rbp-B8h]
  __int64 v101; // [rsp+58h] [rbp-B0h]
  __int64 v102; // [rsp+60h] [rbp-A8h]
  _BYTE v103[8]; // [rsp+68h] [rbp-A0h] BYREF
  _BYTE v104[8]; // [rsp+70h] [rbp-98h] BYREF
  _BYTE v105[8]; // [rsp+78h] [rbp-90h] BYREF
  _BYTE v106[8]; // [rsp+80h] [rbp-88h] BYREF
  _BYTE v107[13]; // [rsp+88h] [rbp-80h] BYREF
  unsigned int v108; // [rsp+95h] [rbp-73h]
  _UNKNOWN *retaddr; // [rsp+170h] [rbp+68h] BYREF

  result = &retaddr;
  v100 = -2;
  if ( !*(_QWORD *)(a1 + 1752) )
    return result;
  if ( !*(_QWORD *)(a1 + 1768) )
    return result;
  if ( !*(_QWORD *)(a1 + 1784) )
    return result;
  if ( !*(_QWORD *)(a1 + 2488) )
    return result;
  v3 = sub_1444D2BB0(a1);
  result = (_UNKNOWN **)sub_1444D2CB0(v3);
  v5 = result;
  if ( !result )
    return result;
  v6 = sub_1444D2BB0(v4);
  v7 = sub_1444D2A70(v6);
  v9 = sub_1444D2BB0(v8);
  v10 = sub_1444D2BA0(v9);
  v11 = *(_DWORD *)v5;
  if ( *(_DWORD *)v5 == 1 )
  {
    sub_146EECBB0(*(_QWORD *)(a1 + 1768), (unsigned int)*((unsigned __int16 *)v5 + 2) + 34);
    sub_146EED4A0(*(_QWORD *)(a1 + 1768));
    v12 = *(_QWORD *)(a1 + 1784);
    if ( v7 <= 0 )
    {
      sub_146EECBB0(v12, 23);
      v13 = 0;
    }
    else
    {
      sub_146EECBB0(v12, 24);
      sub_143FD16D0(*(_QWORD *)(a1 + 1800), (unsigned int)v7);
      LOBYTE(v13) = 1;
    }
    (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 1800) + 16LL))(*(_QWORD *)(a1 + 1800), v13);
    v15 = *(_QWORD *)(a1 + 1816);
    if ( v15 )
    {
      LOBYTE(v14) = 1;
LABEL_24:
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v15 + 24LL))(v15, v14);
    }
  }
  else if ( v11 == 2 )
  {
    sub_146EECBB0(*(_QWORD *)(a1 + 1768), (unsigned int)*((unsigned __int16 *)v5 + 2) + 37);
    sub_146EED4A0(*(_QWORD *)(a1 + 1768));
    v16 = *(_QWORD *)(a1 + 1784);
    v17 = 2 * *(_DWORD *)((char *)v5 + 6);
    if ( v7 <= 0 )
    {
      sub_146EECBB0(v16, (unsigned int)(v17 + 25));
      v18 = 0;
    }
    else
    {
      sub_146EECBB0(v16, (unsigned int)(v17 + 26));
      sub_143FD16D0(*(_QWORD *)(a1 + 1800), (unsigned int)v7);
      LOBYTE(v18) = 1;
    }
    (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 1800) + 16LL))(*(_QWORD *)(a1 + 1800), v18);
    v15 = *(_QWORD *)(a1 + 1816);
    if ( v15 )
    {
      LOBYTE(v14) = 1;
      goto LABEL_24;
    }
  }
  else if ( v11 == 3 )
  {
    sub_146EECBB0(*(_QWORD *)(a1 + 1768), (unsigned int)*((unsigned __int16 *)v5 + 2) + 37);
    sub_146EED4A0(*(_QWORD *)(a1 + 1768));
    v19 = *(_QWORD *)(a1 + 1784);
    if ( v10 <= 0 )
    {
      sub_146EECBB0(v19, 31);
      v20 = 0;
    }
    else
    {
      sub_146EECBB0(v19, 32);
      sub_143FD16D0(*(_QWORD *)(a1 + 1800), (unsigned int)v10);
      LOBYTE(v20) = 1;
    }
    (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 1800) + 16LL))(*(_QWORD *)(a1 + 1800), v20);
    v15 = *(_QWORD *)(a1 + 1816);
    if ( v15 )
    {
      v14 = 0;
      goto LABEL_24;
    }
  }
  v21 = sub_146EC9470(*(_QWORD *)(a1 + 1768));
  v22 = *(float *)&v21;
  v23 = sub_146EC9470(*(_QWORD *)(a1 + 1752));
  v24 = (int)(float)(v22 - *(float *)&v23) + 100;
  v26 = _mm_cvtsi32_si128(sub_146EE4DA0(*(_QWORD *)(a1 + 2488)));
  v27 = 0;
  if ( v24 > 0 )
    v27 = (unsigned int)v24;
  LOBYTE(v25) = 1;
  (*(void (__fastcall **)(_QWORD, __int64, __int64))(**(_QWORD **)(a1 + 2488) + 664LL))(
    *(_QWORD *)(a1 + 2488),
    v27,
    v25);
  v91 = 0;
  v93 = (float)dword_14DC6D900;
  v28 = _mm_cvtepi32_ps(v26).m128_f32[0];
  v98 = v28;
  v92 = v28;
  v29 = sub_146EC9470(*(_QWORD *)(a1 + 1768));
  v94 = *(float *)&v29 + v28;
  sub_146ED3220(*(_QWORD *)(a1 + 1768), &v91);
  v30 = *(_QWORD *)(a1 + 1768);
  v31 = *(void (__fastcall **)(__int64))(*(_QWORD *)v30 + 112LL);
  sub_142757420(*(_QWORD *)(a1 + 1752));
  sub_146ECA0C0(*(_QWORD *)(a1 + 1752));
  v31(v30);
  v33 = sub_1444D2BB0(v32);
  result = (_UNKNOWN **)sub_1444D2AE0(v33);
  v34 = result;
  v99 = result;
  if ( result )
  {
    v35 = 0;
    v97 = 0;
    v36 = result[1];
    v37 = (_QWORD *)*v36;
    if ( (_QWORD *)*v36 != v36 )
    {
      v38 = (_QWORD *)(a1 + 2152);
      while ( 1 )
      {
        v39 = *((_DWORD *)v37 + 10);
        v96 = v39;
        v40 = *((_DWORD *)v37 + 11);
        v41 = *((unsigned __int16 *)v37 + 24) - 1;
        v95 = v41;
        if ( *(v38 - 40) )
        {
          if ( *v38 && v40 >= 2 )
            break;
        }
LABEL_80:
        result = (_UNKNOWN **)v37[2];
        if ( *((_BYTE *)result + 25) )
        {
          for ( result = (_UNKNOWN **)v37[1]; !*((_BYTE *)result + 25); result = (_UNKNOWN **)result[1] )
          {
            if ( v37 != (_QWORD *)result[2] )
              break;
            v37 = result;
          }
          v37 = result;
        }
        else
        {
          v37 = (_QWORD *)v37[2];
          v87 = *result;
          if ( !*((_BYTE *)*result + 25) )
          {
            do
            {
              v37 = v87;
              result = (_UNKNOWN **)*v87;
              v87 = result;
            }
            while ( !*((_BYTE *)result + 25) );
          }
        }
        if ( v37 == (_QWORD *)v34[1] )
          goto LABEL_88;
      }
      v42 = v39;
      v43 = qword_14E664BF8;
      if ( !qword_14E664BF8 )
      {
        v44 = sub_146E8BA20(2792);
        v101 = v44;
        if ( v44 )
          v45 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v44);
        else
          v45 = 0;
        qword_14E664BF8 = (__int64)v45;
        (**v45)(v45);
        v42 = *((unsigned int *)v37 + 10);
        v43 = qword_14E664BF8;
      }
      if ( (unsigned __int8)sub_1444D2F60(v43, v42, v34) )
        LOBYTE(v46) = 1;
      else
        v46 = 0;
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*(v38 - 40) + 24LL))(*(v38 - 40), v46);
      if ( !(unsigned __int8)sub_146ECA4A0(*(v38 - 40)) )
      {
        v59 = qword_14E664BF8;
        if ( !qword_14E664BF8 )
        {
          v60 = sub_146E8BA20(2792);
          v102 = v60;
          if ( v60 )
            v61 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v60);
          else
            v61 = 0;
          qword_14E664BF8 = (__int64)v61;
          (**v61)(v61);
          v59 = qword_14E664BF8;
        }
        v62 = sub_1444D2F30(v59, *((unsigned int *)v37 + 10));
        v63 = 3 * v40;
        if ( v62 )
          v64 = (unsigned int)(v63 - 4);
        else
          v64 = (unsigned int)(v63 - 6);
        sub_146EECBB0(*(v38 - 40), v64);
        goto LABEL_64;
      }
      v47 = sub_146ECFE30(*(v38 - 40));
      v48 = *(v38 - 40);
      if ( v47 )
      {
        sub_146EECBB0(v48, (unsigned int)(v40 + 16));
        v49 = 24;
      }
      else
      {
        if ( !(unsigned __int8)sub_146ED0010(v48) )
        {
          v51 = 0;
          if ( (*(_DWORD *)(a1 + 2508) & 2) != 0 )
          {
            v52 = sub_146E9F840(a1 + 2504);
            v53 = sub_140193D40(a1 + 2504);
            if ( (unsigned __int8)sub_146E9FA80(a1 + 2504) )
            {
              v52 = v53;
              sub_146E9FBD0(a1 + 2504, 1000, 0);
            }
            if ( v52 >= 500 )
            {
              v54 = (unsigned int)(v52 - 500);
              v55 = 1;
              v56 = 255;
            }
            else
            {
              v54 = (unsigned int)v52;
              v55 = 255;
              v56 = 1;
            }
            v57 = sub_146EA1750(v56, v55, v54, 500);
            v39 = v96;
            v41 = v95;
            v51 = v57;
          }
          v58 = 3 * v40;
          sub_146EECBB0(*(v38 - 40), (unsigned int)(v58 - 4));
          sub_146EECBB0(*v38, (unsigned int)(v58 - 5));
          sub_146EED4A0(*v38);
          (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v38 + 376LL))(*v38, v51);
          goto LABEL_64;
        }
        v50 = 3 * v40;
        sub_146EECBB0(*(v38 - 40), (unsigned int)(v50 - 5));
        v49 = (unsigned int)(v50 - 4);
      }
      sub_146EECBB0(*v38, v49);
      sub_146EED4A0(*v38);
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*v38 + 376LL))(*v38, 255);
LABEL_64:
      sub_146ECA0C0(*(_QWORD *)(a1 + 1752));
      v65 = sub_142757420(*(_QWORD *)(a1 + 1752));
      v66 = (float)(*(float *)&v65 + (float)(100 * (v41 / 5))) - v28;
      v67 = sub_142757420(*(_QWORD *)(a1 + 1752));
      v68 = *(float *)&v67;
      v69 = sub_146EC9470(*(_QWORD *)(a1 + 1752));
      v70 = v68 + *(float *)&v69;
      sub_146EC9EB0(*(v38 - 40), v103);
      v71 = (float)(100.0 - *(float *)(sub_146EC9EB0(*(v38 - 40), v104) + 4)) * 0.5;
      sub_146EC9EB0(*v38, v105);
      v72 = (float)(100.0 - *(float *)(sub_146EC9EB0(*v38, v106) + 4)) * 0.5;
      v91 = 0;
      v93 = (float)dword_14DC6D900;
      v73 = sub_142757420(*(_QWORD *)(a1 + 1752));
      if ( (float)(v71 + v66) >= *(float *)&v73 )
      {
        v92 = 0.0;
      }
      else
      {
        v74 = sub_142757420(*(_QWORD *)(a1 + 1752));
        v92 = *(float *)&v74 - (float)(v71 + v66);
      }
      if ( (float)((float)(v66 + 100.0) - v71) <= v70 )
        v94 = 100.0;
      else
        v94 = v70 - v66;
      sub_146ED3220(*(v38 - 40), &v91);
      (*(void (__fastcall **)(_QWORD))(*(_QWORD *)*(v38 - 40) + 112LL))(*(v38 - 40));
      LOBYTE(v75) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*(v38 - 40) + 16LL))(*(v38 - 40), v75);
      v91 = 0;
      v93 = (float)dword_14DC6D900;
      v76 = sub_142757420(*(_QWORD *)(a1 + 1752));
      if ( (float)(v72 + v66) >= *(float *)&v76 )
      {
        v92 = 0.0;
      }
      else
      {
        v77 = sub_142757420(*(_QWORD *)(a1 + 1752));
        v92 = *(float *)&v77 - (float)(v72 + v66);
      }
      if ( (float)((float)(v66 + 100.0) - v72) <= v70 )
        v94 = 100.0;
      else
        v94 = v70 - v66;
      sub_146ED3220(*v38, &v91);
      (*(void (__fastcall **)(_QWORD))(*(_QWORD *)*v38 + 112LL))(*v38);
      v78 = *v38;
      v79 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)*v38 + 16LL);
      v80 = sub_146ECA4A0(*(v38 - 40));
      v79(v78, v80);
      if ( sub_146ECFD90(*(v38 - 40)) && !(unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 6408)) )
      {
        v108 = v39;
        v82 = sub_146D74000(v81);
        sub_146D746E0(v82, 1615);
        v84 = sub_146D74000(v83);
        sub_146D75B10(v84, v107, 17);
        sub_146D75AF0(v86, v85);
      }
      v35 = ++v97;
      v38 += 2;
      v28 = v98;
      v34 = v99;
      goto LABEL_80;
    }
LABEL_88:
    if ( v35 < 20LL )
    {
      result = (_UNKNOWN **)(16LL * v35);
      v88 = (_UNKNOWN **)((char *)result + a1 + 2152);
      v89 = 20LL - v35;
      do
      {
        v90 = *(v88 - 40);
        if ( v90 )
          result = (_UNKNOWN **)(*(__int64 (__fastcall **)(__int64, _QWORD, _UNKNOWN **))(*(_QWORD *)v90 + 16LL))(
                                  v90,
                                  0,
                                  v34);
        if ( *v88 )
          result = (_UNKNOWN **)(*(__int64 (__fastcall **)(_QWORD, _QWORD, _UNKNOWN **))(*(_QWORD *)*v88 + 16LL))(
                                  *v88,
                                  0,
                                  v34);
        v88 += 2;
        --v89;
      }
      while ( v89 );
    }
  }
  return result;
}

