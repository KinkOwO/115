// caller_sub_1444EECA0_0x1444eeca0

__int64 __fastcall sub_1444EECA0(__int64 a1, int a2)
{
  __int64 v2; // r12
  int v4; // ebx
  unsigned int *v5; // rsi
  __int64 **v6; // r13
  __int64 v7; // rdx
  __int64 result; // rax
  __int64 v9; // rcx
  __int64 v10; // rax
  __int64 v11; // rax
  __int64 v12; // rax
  int v13; // edi
  __int64 *v14; // r9
  __int64 *v15; // rax
  __int64 *v16; // rdx
  unsigned int v17; // r8d
  __int64 *v18; // rcx
  char v19; // dl
  unsigned int v20; // ebx
  int v21; // r14d
  __int64 v22; // rcx
  __int64 v23; // rcx
  unsigned int v24; // ebx
  __int64 v25; // rax
  unsigned int *v26; // rax
  signed int v27; // edx
  __int64 *v28; // r9
  __int64 *v29; // rax
  __int64 *v30; // r8
  __int64 *v31; // rcx
  char v32; // al
  char v33; // r8
  unsigned int v34; // ebx
  __int64 v35; // r8
  int v36; // ecx
  unsigned __int64 v37; // rdx
  __int64 v38; // rax
  __int64 v39; // rax
  __int64 v40; // r14
  unsigned int v41; // edx
  __int64 *v42; // r9
  __int64 *v43; // rax
  __int64 *v44; // r8
  char v45; // r8
  unsigned int v46; // ebx
  __int64 v47; // rax
  volatile signed __int32 *v48; // rcx
  volatile signed __int32 *v49; // rcx
  __int64 v50; // rax
  __int64 v51; // rax
  __int64 v52; // rax
  __int64 v53; // rax
  __int64 v54; // rax
  __int64 v55; // rdx
  __int64 v56; // r14
  __int64 *v57; // rax
  __int64 *v58; // rcx
  _QWORD *v59; // rcx
  __int64 v60; // rsi
  __int64 v61; // rbx
  unsigned __int64 v62; // rdx
  __int64 v63; // r14
  unsigned int *v64; // rdx
  int v65; // r12d
  unsigned __int64 *v66; // r13
  __int64 v67; // rbx
  unsigned __int64 v68; // r14
  __int64 v69; // r8
  __int64 v70; // rax
  __int64 v71; // rdx
  int v72; // ecx
  __int64 v73; // r8
  unsigned __int64 v74; // rcx
  float v75; // xmm0_4
  __int64 v76; // rcx
  float v77; // xmm1_4
  __int64 v78; // rax
  __int64 v79; // r8
  unsigned __int64 v80; // rcx
  int v81; // eax
  __int64 *v82; // r9
  __int64 v83; // rcx
  __int64 v84; // rax
  __int64 v85; // r8
  unsigned __int64 v86; // rdx
  __int64 v87; // rax
  __int128 v88; // [rsp+20h] [rbp-B9h] BYREF
  unsigned int *v89; // [rsp+30h] [rbp-A9h]
  __int128 v90; // [rsp+40h] [rbp-99h] BYREF
  __int128 v91; // [rsp+50h] [rbp-89h] BYREF
  __int64 v92; // [rsp+60h] [rbp-79h]
  unsigned int v93; // [rsp+68h] [rbp-71h] BYREF
  unsigned int v94; // [rsp+6Ch] [rbp-6Dh] BYREF
  int v95; // [rsp+70h] [rbp-69h] BYREF
  int v96; // [rsp+74h] [rbp-65h]
  char v97[8]; // [rsp+78h] [rbp-61h] BYREF
  __int128 v98; // [rsp+80h] [rbp-59h]
  __int64 v99; // [rsp+90h] [rbp-49h]
  int v100; // [rsp+98h] [rbp-41h] BYREF
  __int128 v101; // [rsp+A0h] [rbp-39h] BYREF
  __int64 v102; // [rsp+B0h] [rbp-29h]
  __int128 v103; // [rsp+C0h] [rbp-19h]
  __int64 v104; // [rsp+D0h] [rbp-9h]
  __int128 v105; // [rsp+D8h] [rbp-1h]
  unsigned __int16 v106; // [rsp+140h] [rbp+67h] BYREF
  int v107; // [rsp+148h] [rbp+6Fh] BYREF
  unsigned __int16 v108; // [rsp+150h] [rbp+77h] BYREF
  unsigned int v109; // [rsp+158h] [rbp+7Fh] BYREF

  v107 = a2;
  v104 = -2;
  v2 = a2;
  v4 = 0;
  v96 = 0;
  v88 = 0u;
  v89 = 0;
  v5 = 0;
  v6 = (__int64 **)(a1 + 296);
  v7 = *(_QWORD *)(a1 + 296);
  result = *(_QWORD *)(v7 + 8);
  v9 = v7;
  while ( !*(_BYTE *)(result + 25) )
  {
    if ( *(_DWORD *)(result + 32) >= (int)v2 )
    {
      v9 = result;
      result = *(_QWORD *)result;
    }
    else
    {
      result = *(_QWORD *)(result + 16);
    }
  }
  if ( !*(_BYTE *)(v9 + 25) && (int)v2 >= *(_DWORD *)(v9 + 32) && v9 != v7 )
  {
    result = *(_QWORD *)(v9 + 40);
    *(_QWORD *)(v9 + 48) = result;
  }
  switch ( v2 )
  {
    case 0LL:
      sub_146EA0BA0(&v109);
      v10 = sub_1444EBD90(a1, 0, v109);
      if ( v10 && !(unsigned __int8)sub_1444EC2C0(v10) )
      {
        v107 = v109;
        sub_140154010(&v88, 0, &v107);
        v5 = (unsigned int *)*((_QWORD *)&v88 + 1);
      }
      sub_146EA0BA0(&v93);
      v11 = sub_1444EBD90(a1, 0, v93);
      if ( v11 && !(unsigned __int8)sub_1444EC2C0(v11) )
      {
        v107 = v93;
        if ( v5 == v89 )
        {
          sub_140154010(&v88, v5, &v107);
          v5 = (unsigned int *)*((_QWORD *)&v88 + 1);
        }
        else
        {
          *v5++ = v93;
          *((_QWORD *)&v88 + 1) = v5;
        }
      }
      sub_146EA0BA0(&v94);
      v12 = sub_1444EBD90(a1, 0, v94);
      if ( v12 && !(unsigned __int8)sub_1444EC2C0(v12) )
      {
        v107 = v94;
        if ( v5 == v89 )
        {
          sub_140154010(&v88, v5, &v107);
          v5 = (unsigned int *)*((_QWORD *)&v88 + 1);
        }
        else
        {
          *v5++ = v94;
          *((_QWORD *)&v88 + 1) = v5;
        }
      }
      sub_1401CA430(a1 + 312);
      result = sub_146EA1920(&v106);
      v13 = 0;
      v9 = v106;
      if ( !v106 )
        goto LABEL_51;
      while ( 1 )
      {
        sub_146EA0BA0(&v95);
        v14 = *(__int64 **)(a1 + 136);
        v15 = (__int64 *)v14[1];
        v16 = v14;
        v17 = v95;
        while ( !*((_BYTE *)v15 + 25) )
        {
          if ( *((_DWORD *)v15 + 7) >= v95 )
          {
            v16 = v15;
            v15 = (__int64 *)*v15;
          }
          else
          {
            v15 = (__int64 *)v15[2];
          }
        }
        if ( *((_BYTE *)v16 + 25) || v95 < *((_DWORD *)v16 + 7) )
          v16 = *(__int64 **)(a1 + 136);
        result = (__int64)(v16 + 4);
        v18 = 0;
        if ( v16 != v14 )
          v18 = v16 + 4;
        if ( !v18 )
          goto LABEL_49;
        result = *((unsigned __int8 *)v18 + 12);
        if ( !(_BYTE)result || *((_DWORD *)v18 + 2) )
        {
          v19 = *((_BYTE *)v18 + 20);
          if ( !v19 || *((_DWORD *)v18 + 4) )
          {
            v20 = 0;
            if ( (_BYTE)result )
              v20 = *((_DWORD *)v18 + 2);
            if ( v19 )
            {
              result = *((unsigned int *)v18 + 4);
              if ( *((_DWORD *)v18 + 2) < (unsigned int)result )
                v20 = *((_DWORD *)v18 + 4);
            }
            if ( !v20 )
              goto LABEL_49;
            result = sub_145A11A50();
            if ( (unsigned int)result > v20 )
              goto LABEL_49;
            v17 = v95;
          }
        }
        result = sub_1444E8ED0(a1, v17);
LABEL_49:
        ++v13;
        v9 = v106;
        if ( v13 >= v106 )
        {
          v6 = (__int64 **)(a1 + 296);
LABEL_51:
          if ( !(_WORD)v9 )
            result = sub_1444E8ED0(a1, 80000);
          goto LABEL_155;
        }
      }
    case 1LL:
      sub_146EA1920(&v108);
      v21 = 0;
      if ( !v108 )
        goto LABEL_86;
      do
      {
        sub_146EA0BA0(&v109);
        if ( v109 )
        {
          if ( sub_145EFAFB0(v22) )
          {
            v24 = v109;
            v25 = sub_145EFAFB0(v23);
            v26 = (unsigned int *)(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v25 + 7656LL))(v25);
            v27 = sub_1473A1120(a1 + 1328, *v26, v24);
            v109 = v27;
          }
          else
          {
            v27 = v109;
          }
          v28 = *(__int64 **)(a1 + 152);
          v29 = (__int64 *)v28[1];
          v30 = v28;
          while ( !*((_BYTE *)v29 + 25) )
          {
            if ( *((_DWORD *)v29 + 7) >= v27 )
            {
              v30 = v29;
              v29 = (__int64 *)*v29;
            }
            else
            {
              v29 = (__int64 *)v29[2];
            }
          }
          if ( *((_BYTE *)v30 + 25) || v27 < *((_DWORD *)v30 + 7) )
            v30 = *(__int64 **)(a1 + 152);
          v31 = 0;
          if ( v30 != v28 )
            v31 = v30 + 4;
          if ( v31 )
          {
            v32 = *((_BYTE *)v31 + 12);
            if ( v32 && !*((_DWORD *)v31 + 2) )
              goto LABEL_81;
            v33 = *((_BYTE *)v31 + 20);
            if ( v33 )
            {
              if ( !*((_DWORD *)v31 + 4) )
                goto LABEL_81;
            }
            v34 = 0;
            if ( v32 )
              v34 = *((_DWORD *)v31 + 2);
            if ( v33 && *((_DWORD *)v31 + 2) < *((_DWORD *)v31 + 4) )
              v34 = *((_DWORD *)v31 + 4);
            if ( v34 && (unsigned int)sub_145A11A50() <= v34 )
            {
              v27 = v109;
LABEL_81:
              LODWORD(v90) = v27;
              if ( v5 == v89 )
              {
                sub_140154010(&v88, v5, &v90);
                v5 = (unsigned int *)*((_QWORD *)&v88 + 1);
              }
              else
              {
                *v5++ = v27;
                *((_QWORD *)&v88 + 1) = v5;
              }
            }
          }
        }
        ++v21;
      }
      while ( v21 < v108 );
      v6 = (__int64 **)(a1 + 296);
      v4 = 0;
LABEL_86:
      v91 = 0;
      v92 = 0;
      v107 = 0;
      sub_146EA1920(&v107);
      v36 = v107;
      if ( v107 > (unsigned __int64)((v92 - (__int64)v91) >> 2) )
      {
        if ( (unsigned __int64)v107 > 0x3FFFFFFFFFFFFFFFLL )
          sub_14014E270(v107, v107);
        sub_1402B0370(&v91, v107, v35);
        v36 = v107;
      }
      if ( v36 > 0 )
      {
        do
        {
          LODWORD(v90) = 0;
          sub_146EA0BA0(&v90);
          if ( (int)v90 > 0 )
          {
            if ( *((_QWORD *)&v91 + 1) == v92 )
            {
              sub_140154010(&v91, *((_QWORD *)&v91 + 1), &v90);
            }
            else
            {
              **((_DWORD **)&v91 + 1) = v90;
              *((_QWORD *)&v91 + 1) += 4LL;
            }
          }
          ++v4;
        }
        while ( v4 < v107 );
      }
      *(_QWORD *)(a1 + 1160) = *(_QWORD *)(a1 + 1152);
      sub_143D855D0(a1 + 1152, v91, *((_QWORD *)&v91 + 1));
      *(_QWORD *)(a1 + 1184) = *(_QWORD *)(a1 + 1176);
      result = sub_143D855D0(a1 + 1176, v91, *((_QWORD *)&v91 + 1));
      v9 = v91;
      if ( (_QWORD)v91 )
      {
        v37 = 4 * ((v92 - (__int64)v91) >> 2);
        if ( v37 >= 0x1000 )
        {
          v37 += 39LL;
          v9 = *(_QWORD *)(v91 - 8);
          if ( (unsigned __int64)(v91 - v9 - 8) > 0x1F )
            sub_148AAF304(v9, v37);
        }
        result = sub_146E9F3A0(v9, v37);
        v91 = 0;
        v92 = 0;
      }
LABEL_155:
      v56 = v88;
      if ( (unsigned int *)v88 != v5 )
      {
        v57 = (__int64 *)(*v6)[1];
        v58 = *v6;
        while ( !*((_BYTE *)v57 + 25) )
        {
          if ( *((_DWORD *)v57 + 8) >= (int)v2 )
          {
            v58 = v57;
            v57 = (__int64 *)*v57;
          }
          else
          {
            v57 = (__int64 *)v57[2];
          }
        }
        if ( *((_BYTE *)v58 + 25) || (int)v2 < *((_DWORD *)v58 + 8) || v58 == *v6 )
        {
          v100 = v2;
          v101 = 0;
          v102 = 0;
          v60 = (__int64)v5 - v88;
          *(_QWORD *)&v101 = sub_140157580(&v101, v60 >> 2);
          *((_QWORD *)&v101 + 1) = v101;
          v102 = v101 + 4 * (v60 >> 2);
          *(_QWORD *)&v90 = &v101;
          v61 = v101;
          sub_148AA1E60(v101, v56, v60);
          *((_QWORD *)&v101 + 1) = v61 + 4 * (v60 >> 2);
          *(_QWORD *)&v90 = 0;
          v96 = 16;
          result = sub_1401844D0(v6, &v91, &v100);
          v96 = 0;
          v9 = v101;
          if ( (_QWORD)v101 )
          {
            v62 = 4 * ((v102 - (__int64)v101) >> 2);
            if ( v62 >= 0x1000 )
            {
              v62 += 39LL;
              v9 = *(_QWORD *)(v101 - 8);
              if ( (unsigned __int64)(v101 - v9 - 8) > 0x1F )
                sub_148AAF304(v9, v62);
            }
            result = sub_146E9F3A0(v9, v62);
            v101 = 0;
            v102 = 0;
          }
        }
        else
        {
          v59 = v58 + 5;
          v59[1] = *v59;
          result = sub_143D855D0(v59, v56, v5);
        }
      }
LABEL_209:
      if ( v56 )
      {
        v86 = ((unsigned __int64)v89 - v56) & 0xFFFFFFFFFFFFFFFCuLL;
        v87 = v56;
        if ( v86 >= 0x1000 )
        {
          v86 += 39LL;
          v56 = *(_QWORD *)(v56 - 8);
          if ( (unsigned __int64)(v87 - v56 - 8) > 0x1F )
            sub_148AAF304(v9, v86);
        }
        result = sub_146E9F3A0(v56, v86);
        v88 = 0;
        v89 = 0;
      }
      return result;
    case 2LL:
      sub_146EA0BA0(&v109);
      result = sub_1444EBD90(a1, 2, v109);
      if ( !result )
        goto LABEL_208;
      result = sub_1444EC2C0(result);
      if ( (_BYTE)result )
        goto LABEL_208;
      v107 = v109;
      sub_140154010(&v88, 0, &v107);
      v38 = sub_143C61150();
      sub_1447EF2F0(v38, v109);
      v39 = sub_143C61150();
      result = sub_1447EF3B0(v39, v109, 0);
      goto LABEL_154;
    case 3LL:
      v40 = 4;
      while ( 1 )
      {
        result = sub_146EA0BA0(&v109);
        v41 = v109;
        if ( v109 )
        {
          v42 = *(__int64 **)(a1 + 184);
          v43 = (__int64 *)v42[1];
          v44 = v42;
          while ( !*((_BYTE *)v43 + 25) )
          {
            if ( *((_DWORD *)v43 + 7) >= (signed int)v109 )
            {
              v44 = v43;
              v43 = (__int64 *)*v43;
            }
            else
            {
              v43 = (__int64 *)v43[2];
            }
          }
          if ( *((_BYTE *)v44 + 25) || (signed int)v109 < *((_DWORD *)v44 + 7) )
            v44 = *(__int64 **)(a1 + 184);
          result = (__int64)(v44 + 4);
          v9 = 0;
          if ( v44 != v42 )
            v9 = (__int64)(v44 + 4);
          if ( !v9 )
            goto LABEL_132;
          result = *(unsigned __int8 *)(v9 + 12);
          if ( !(_BYTE)result || *(_DWORD *)(v9 + 8) )
          {
            v45 = *(_BYTE *)(v9 + 20);
            if ( !v45 || *(_DWORD *)(v9 + 16) )
            {
              v46 = 0;
              if ( (_BYTE)result )
                v46 = *(_DWORD *)(v9 + 8);
              if ( v45 )
              {
                result = *(unsigned int *)(v9 + 16);
                if ( *(_DWORD *)(v9 + 8) < (unsigned int)result )
                  v46 = *(_DWORD *)(v9 + 16);
              }
              if ( !v46 )
                goto LABEL_132;
              result = sub_145A11A50();
              if ( (unsigned int)result > v46 )
                goto LABEL_132;
              v41 = v109;
            }
          }
          LODWORD(v90) = v41;
          if ( v5 == v89 )
          {
LABEL_131:
            result = sub_140154010(&v88, v5, &v90);
            v5 = (unsigned int *)*((_QWORD *)&v88 + 1);
            goto LABEL_132;
          }
          *v5++ = v41;
          *((_QWORD *)&v88 + 1) = v5;
        }
        else
        {
          LODWORD(v90) = 0;
          if ( v5 == v89 )
            goto LABEL_131;
          *v5++ = 0;
          *((_QWORD *)&v88 + 1) = v5;
        }
LABEL_132:
        if ( !--v40 )
        {
          LODWORD(v2) = v107;
          goto LABEL_155;
        }
      }
    case 4LL:
      sub_146EA0BA0(&v109);
      v47 = sub_14603E2A0(v109, 0);
      sub_14057E020(v97, v47);
      if ( !*((_QWORD *)&v98 + 1) || !*(_DWORD *)(*((_QWORD *)&v98 + 1) + 8LL) || !v99 )
      {
        result = sub_140422B30(v97);
        goto LABEL_208;
      }
      v90 = 0;
      v91 = v98;
      v48 = (volatile signed __int32 *)*((_QWORD *)&v98 + 1);
      v98 = 0u;
      if ( *((_QWORD *)&v91 + 1) && _InterlockedExchangeAdd(v48 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v48 + 8LL))(v48);
      v99 = 0;
      v105 = 0;
      v103 = v98;
      v49 = (volatile signed __int32 *)*((_QWORD *)&v98 + 1);
      v98 = 0u;
      if ( *((_QWORD *)&v103 + 1) && _InterlockedExchangeAdd(v49 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v49 + 8LL))(v49);
      v99 = 0;
      v107 = v109;
      sub_140154010(&v88, 0, &v107);
      result = sub_140422B30(v97);
      goto LABEL_154;
    case 6LL:
      sub_146EA0BA0(&v109);
      v50 = sub_1444EBD90(a1, 2, v109);
      if ( v50 )
      {
        result = sub_1444EC2C0(v50);
        if ( (_BYTE)result )
          goto LABEL_208;
        v107 = v109;
        sub_140154010(&v88, 0, &v107);
        v51 = sub_143C61150();
        sub_142581F20(v51, v109);
        v52 = sub_143C61150();
        result = sub_1447EF380(v52, v109, 0);
      }
      else
      {
        sub_140154010(&v88, 0, &unk_14A317CF0);
        v53 = sub_143C61150();
        sub_142581F20(v53, 99999999);
        v54 = sub_143C61150();
        result = sub_1447EF380(v54, 99999999, 0);
      }
LABEL_154:
      v5 = (unsigned int *)*((_QWORD *)&v88 + 1);
      goto LABEL_155;
    case 7LL:
      sub_146EA0BA0(&v109);
      v55 = 7;
      goto LABEL_151;
    case 8LL:
      sub_146EA0BA0(&v109);
      v55 = 8;
LABEL_151:
      result = sub_1444EBD90(a1, v55, v109);
      if ( !result )
        goto LABEL_208;
      result = sub_1444EC2C0(result);
      if ( (_BYTE)result )
        goto LABEL_208;
      v107 = v109;
      result = sub_140154010(&v88, 0, &v107);
      goto LABEL_154;
    case 9LL:
      *(_QWORD *)(a1 + 120) = *(_QWORD *)(a1 + 112);
      sub_1402B0ED0(a1 + 24);
      v63 = 4;
      do
      {
        v109 = 0;
        sub_146EA0BA0(&v109);
        v64 = *(unsigned int **)(a1 + 120);
        if ( v64 == *(unsigned int **)(a1 + 128) )
        {
          sub_140154010(a1 + 112, v64, &v109);
        }
        else
        {
          *v64 = v109;
          *(_QWORD *)(a1 + 120) += 4LL;
        }
        --v63;
      }
      while ( v63 );
      v107 = 0;
      sub_146EA1920(&v107);
      v65 = 0;
      if ( v107 <= 0 )
        goto LABEL_207;
      v66 = (unsigned __int64 *)(a1 + 32);
      break;
    default:
      goto LABEL_208;
  }
  while ( 1 )
  {
    sub_146EA0BA0(&v109);
    *(_QWORD *)&v91 = a1 + 32;
    v67 = sub_146E8BA20(24);
    *((_QWORD *)&v91 + 1) = v67;
    *(_DWORD *)(v67 + 16) = v109;
    v68 = 0x100000001B3LL
        * (*(unsigned __int8 *)(v67 + 19)
         ^ (0x100000001B3LL
          * (*(unsigned __int8 *)(v67 + 18)
           ^ (0x100000001B3LL
            * (*(unsigned __int8 *)(v67 + 17)
             ^ (0x100000001B3LL * (*(unsigned __int8 *)(v67 + 16) ^ 0xCBF29CE484222325uLL)))))));
    v69 = *(_QWORD *)(a1 + 48);
    v70 = *(_QWORD *)(v69 + 16 * (v68 & *(_QWORD *)(a1 + 72)) + 8);
    v71 = *v66;
    if ( v70 != *v66 )
    {
      v72 = *(_DWORD *)(v67 + 16);
      if ( v72 == *(_DWORD *)(v70 + 16) )
      {
LABEL_180:
        sub_146E9F3A0(v67, 24);
        goto LABEL_181;
      }
      while ( v70 != *(_QWORD *)(v69 + 16 * (v68 & *(_QWORD *)(a1 + 72))) )
      {
        v70 = *(_QWORD *)(v70 + 8);
        if ( v72 == *(_DWORD *)(v70 + 16) )
          goto LABEL_180;
      }
      v71 = v70;
    }
    v73 = *(_QWORD *)(a1 + 40);
    if ( v73 == 0xAAAAAAAAAAAAAAALL )
      sub_14883BB54("unordered_map/set too long");
    v74 = v73 + 1;
    if ( v73 + 1 < 0 )
      v75 = (float)(int)(v74 & 1 | (v74 >> 1)) + (float)(int)(v74 & 1 | (v74 >> 1));
    else
      v75 = (float)(int)v74;
    v76 = *(_QWORD *)(a1 + 80);
    if ( v76 < 0 )
    {
      v78 = *(_QWORD *)(a1 + 80) & 1LL | ((unsigned __int64)v76 >> 1);
      v77 = (float)(int)v78 + (float)(int)v78;
    }
    else
    {
      v77 = (float)(int)v76;
    }
    if ( *(float *)(a1 + 24) < (float)(v75 / v77) )
    {
      sub_1407FC8B0(a1 + 24);
      v79 = *(_QWORD *)(a1 + 48);
      v80 = *(_QWORD *)(v79 + 16 * (v68 & *(_QWORD *)(a1 + 72)) + 8);
      if ( v80 == *v66 )
      {
        v90 = *v66;
      }
      else
      {
        v81 = *(_DWORD *)(v67 + 16);
        if ( v81 == *(_DWORD *)(v80 + 16) )
        {
LABEL_197:
          *(_QWORD *)&v90 = *(_QWORD *)v80;
          *((_QWORD *)&v90 + 1) = v80;
        }
        else
        {
          while ( v80 != *(_QWORD *)(v79 + 16 * (v68 & *(_QWORD *)(a1 + 72))) )
          {
            v80 = *(_QWORD *)(v80 + 8);
            if ( v81 == *(_DWORD *)(v80 + 16) )
              goto LABEL_197;
          }
          v90 = v80;
        }
      }
      v103 = v90;
      v73 = *(_QWORD *)(a1 + 40);
      v71 = v90;
    }
    *((_QWORD *)&v91 + 1) = 0;
    v82 = *(__int64 **)(v71 + 8);
    *(_QWORD *)(a1 + 40) = v73 + 1;
    *(_QWORD *)v67 = v71;
    *(_QWORD *)(v67 + 8) = v82;
    *v82 = v67;
    *(_QWORD *)(v71 + 8) = v67;
    v83 = *(_QWORD *)(a1 + 48);
    v84 = 2 * (v68 & *(_QWORD *)(a1 + 72));
    v85 = *(_QWORD *)(v83 + 16 * (v68 & *(_QWORD *)(a1 + 72)));
    if ( v85 == *v66 )
    {
      *(_QWORD *)(v83 + 16 * (v68 & *(_QWORD *)(a1 + 72))) = v67;
LABEL_205:
      *(_QWORD *)(v83 + 8 * v84 + 8) = v67;
      goto LABEL_181;
    }
    if ( v85 == v71 )
    {
      *(_QWORD *)(v83 + 16 * (v68 & *(_QWORD *)(a1 + 72))) = v67;
    }
    else if ( *(__int64 **)(v83 + 16 * (v68 & *(_QWORD *)(a1 + 72)) + 8) == v82 )
    {
      goto LABEL_205;
    }
LABEL_181:
    if ( ++v65 >= v107 )
    {
LABEL_207:
      result = sub_1401C4380(a1 + 88, a1 + 112);
LABEL_208:
      v56 = v88;
      goto LABEL_209;
    }
  }
}

