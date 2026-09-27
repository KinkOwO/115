// ec570caller_sub_14346B650_0x14346b650

__int64 __fastcall sub_14346B650(_QWORD *a1, __int64 a2)
{
  __int64 v2; // r14
  _QWORD *v3; // r15
  int v4; // ebx
  __int64 v5; // rax
  int v6; // r12d
  __int64 v7; // rax
  __int64 v8; // rax
  unsigned int v9; // r13d
  __int64 v10; // rax
  int *v11; // rax
  int *v12; // rbx
  int v13; // esi
  __int64 v14; // rax
  int v15; // r14d
  __int64 v16; // rax
  __int64 v17; // rax
  __int64 v18; // rax
  __int64 v19; // r8
  _QWORD *v20; // r8
  __int64 v21; // rax
  volatile signed __int32 *v22; // rbx
  _QWORD *v23; // r15
  volatile signed __int32 *v24; // rbx
  __int64 v25; // r12
  __int64 *v26; // r14
  __int64 *v27; // rax
  __int64 *v28; // rcx
  __int64 v29; // rax
  __int64 v30; // rbx
  __int64 v31; // rax
  __int64 v32; // rax
  __int64 v33; // rax
  __int64 v34; // r8
  _QWORD *v35; // r8
  __int64 v36; // rax
  volatile signed __int32 *v37; // rbx
  _QWORD *v38; // r15
  volatile signed __int32 *v39; // rbx
  __int64 v40; // r12
  __int64 *v41; // r14
  __int64 *v42; // rax
  __int64 *v43; // rcx
  __int64 v44; // rax
  __int64 v45; // rbx
  volatile signed __int32 *v46; // rbx
  volatile signed __int32 *v47; // rbx
  unsigned __int64 v48; // rdx
  __int64 v49; // rcx
  __int64 v50; // rbx
  __int64 v51; // rax
  __int64 v52; // rax
  __int64 v53; // rax
  __int64 result; // rax
  volatile signed __int32 *v55; // rbx
  volatile signed __int32 *v56; // rbx
  unsigned int v57; // [rsp+20h] [rbp-E0h]
  int v58; // [rsp+20h] [rbp-E0h]
  int v59; // [rsp+20h] [rbp-E0h]
  int v60; // [rsp+24h] [rbp-DCh]
  int v62; // [rsp+30h] [rbp-D0h]
  __int128 v63; // [rsp+38h] [rbp-C8h]
  __int64 v64; // [rsp+50h] [rbp-B0h] BYREF
  volatile signed __int32 *v65; // [rsp+58h] [rbp-A8h]
  int v66; // [rsp+60h] [rbp-A0h]
  __int128 v67; // [rsp+68h] [rbp-98h]
  int v68; // [rsp+78h] [rbp-88h]
  __int128 v69; // [rsp+80h] [rbp-80h]
  __int64 v70; // [rsp+90h] [rbp-70h] BYREF
  volatile signed __int32 *v71; // [rsp+98h] [rbp-68h]
  _QWORD *v72; // [rsp+A0h] [rbp-60h]
  __int64 v73; // [rsp+A8h] [rbp-58h]
  _QWORD *v74; // [rsp+B0h] [rbp-50h]
  __int64 v75; // [rsp+B8h] [rbp-48h]
  __int64 v76; // [rsp+C0h] [rbp-40h] BYREF
  volatile signed __int32 *v77; // [rsp+C8h] [rbp-38h]
  __int64 v78; // [rsp+D0h] [rbp-30h] BYREF
  volatile signed __int32 *v79; // [rsp+D8h] [rbp-28h]
  __int64 v80; // [rsp+E0h] [rbp-20h]
  __int64 v81; // [rsp+E8h] [rbp-18h]
  __int64 v82; // [rsp+F0h] [rbp-10h]
  _BYTE v83[16]; // [rsp+F8h] [rbp-8h] BYREF
  char v84[8]; // [rsp+108h] [rbp+8h] BYREF
  volatile signed __int32 *v85; // [rsp+110h] [rbp+10h]
  __int128 v86; // [rsp+120h] [rbp+20h] BYREF
  _BYTE v87[16]; // [rsp+130h] [rbp+30h] BYREF
  char v88[8]; // [rsp+140h] [rbp+40h] BYREF
  volatile signed __int32 *v89; // [rsp+148h] [rbp+48h]
  __int128 v90; // [rsp+150h] [rbp+50h] BYREF
  _QWORD v91[2]; // [rsp+160h] [rbp+60h] BYREF
  __int64 v92; // [rsp+170h] [rbp+70h]
  unsigned __int64 v93; // [rsp+178h] [rbp+78h]

  v80 = -2;
  v2 = a2;
  v70 = a2;
  v3 = a1;
  v4 = 0;
  v57 = 0;
  v5 = sub_1401E65D0(a2);
  v6 = sub_147BA39E0(v5);
  v62 = v6;
  v7 = sub_1401E65D0(v2);
  v8 = sub_141780030(v7);
  v78 = v8;
  v9 = 0;
  v60 = 0;
  if ( v6 > 0 )
  {
    while ( (int)v8 > (int)v9 )
    {
      v10 = sub_1401E65D0(v2);
      v11 = (int *)sub_147BA3980(v10, (unsigned int)v4);
      v12 = v11;
      if ( v11 )
      {
        if ( v11[1] == -2 )
        {
          v13 = *v11;
          v14 = sub_1401E65D0(v2);
          if ( sub_1444EC570(v14, v13) )
          {
            v15 = v12[2];
            if ( v15 >= 0 )
            {
              v16 = sub_146E8C7D0(&unk_149DE6528);
              v17 = sub_146E8CF20(v83, v16, v9);
              v18 = sub_14014F430(v17);
              v91[0] = 0;
              v92 = 0;
              v93 = 7;
              v19 = -1;
              do
                ++v19;
              while ( *(_WORD *)(v18 + 2 * v19) );
              sub_14014C8D0(v91, v18);
              sub_146E8C910(v83);
              v20 = v91;
              if ( v93 >= 8 )
                v20 = (_QWORD *)v91[0];
              v21 = sub_146EC8E30(v3[262], v84, v20);
              sub_1402425E0(&v64, v21);
              v22 = v85;
              if ( v85 )
              {
                if ( _InterlockedExchangeAdd(v85 + 2, 0xFFFFFFFF) == 1 )
                {
                  (**(void (__fastcall ***)(volatile signed __int32 *))v22)(v22);
                  if ( _InterlockedExchangeAdd(v22 + 3, 0xFFFFFFFF) == 1 )
                    (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v22 + 8LL))(v22);
                }
              }
              if ( v64 && v15 > 0 )
              {
                sub_14501C1F0(v64, (unsigned int)v15);
                v23 = v3 + 285;
                v66 = v13;
                v67 = 0;
                v24 = v65;
                if ( v65 )
                {
                  _InterlockedIncrement(v65 + 2);
                  v24 = v65;
                }
                v25 = v64;
                *(_QWORD *)&v67 = v64;
                *((_QWORD *)&v67 + 1) = v24;
                v58 = v57 | 2;
                v26 = (__int64 *)*v23;
                v27 = *(__int64 **)(*v23 + 8LL);
                *(_QWORD *)&v63 = v27;
                DWORD2(v63) = 0;
                v28 = (__int64 *)*v23;
                while ( !*((_BYTE *)v27 + 25) )
                {
                  *(_QWORD *)&v63 = v27;
                  if ( *((_DWORD *)v27 + 8) >= v13 )
                  {
                    DWORD2(v63) = 1;
                    v28 = v27;
                    v27 = (__int64 *)*v27;
                  }
                  else
                  {
                    DWORD2(v63) = 0;
                    v27 = (__int64 *)v27[2];
                  }
                }
                if ( *((_BYTE *)v28 + 25) || v13 < *((_DWORD *)v28 + 8) )
                {
                  if ( v23[1] == 0x492492492492492LL )
                    sub_14014F360(v28, 0);
                  v72 = v23;
                  v73 = 0;
                  v29 = sub_146E8BA20(56);
                  v81 = v29 + 32;
                  *(_DWORD *)(v29 + 32) = v13;
                  *(_QWORD *)(v29 + 40) = v25;
                  *(_QWORD *)(v29 + 48) = v24;
                  v67 = 0;
                  *(_QWORD *)v29 = v26;
                  *(_QWORD *)(v29 + 8) = v26;
                  *(_QWORD *)(v29 + 16) = v26;
                  *(_WORD *)(v29 + 24) = 0;
                  v73 = 0;
                  v86 = v63;
                  sub_14014F0E0(v23, &v86, v29);
                  v24 = (volatile signed __int32 *)*((_QWORD *)&v67 + 1);
                }
                v57 = v58 & 0xFFFFFFFD;
                if ( v24 )
                {
                  if ( _InterlockedExchangeAdd(v24 + 2, 0xFFFFFFFF) == 1 )
                  {
                    v30 = *((_QWORD *)&v67 + 1);
                    (***((void (__fastcall ****)(_QWORD))&v67 + 1))(*((_QWORD *)&v67 + 1));
                    if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v30 + 12), 0xFFFFFFFF) == 1 )
                      (*(void (__fastcall **)(_QWORD))(**((_QWORD **)&v67 + 1) + 8LL))(*((_QWORD *)&v67 + 1));
                  }
                }
                v3 = a1;
              }
              v31 = sub_146E8C7D0(&unk_149DE6560);
              v32 = sub_146E8CF20(v87, v31, v9);
              v33 = sub_14014F430(v32);
              v34 = -1;
              do
                ++v34;
              while ( *(_WORD *)(v33 + 2 * v34) );
              sub_14014C8D0(v91, v33);
              sub_146E8C910(v87);
              v35 = v91;
              if ( v93 >= 8 )
                v35 = (_QWORD *)v91[0];
              v36 = sub_146EC8E30(v3[262], v88, v35);
              sub_1402424B0(&v76, v36);
              v37 = v89;
              if ( v89 )
              {
                if ( _InterlockedExchangeAdd(v89 + 2, 0xFFFFFFFF) == 1 )
                {
                  (**(void (__fastcall ***)(volatile signed __int32 *))v37)(v37);
                  if ( _InterlockedExchangeAdd(v37 + 3, 0xFFFFFFFF) == 1 )
                    (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v37 + 8LL))(v37);
                }
              }
              v38 = v3 + 281;
              v68 = v13;
              v69 = 0;
              v39 = v77;
              if ( v77 )
              {
                _InterlockedIncrement(v77 + 2);
                v39 = v77;
              }
              v40 = v76;
              *(_QWORD *)&v69 = v76;
              *((_QWORD *)&v69 + 1) = v39;
              v59 = v57 | 8;
              v41 = (__int64 *)*v38;
              v42 = *(__int64 **)(*v38 + 8LL);
              *(_QWORD *)&v63 = v42;
              DWORD2(v63) = 0;
              v43 = (__int64 *)*v38;
              while ( !*((_BYTE *)v42 + 25) )
              {
                *(_QWORD *)&v63 = v42;
                if ( *((_DWORD *)v42 + 8) >= v13 )
                {
                  DWORD2(v63) = 1;
                  v43 = v42;
                  v42 = (__int64 *)*v42;
                }
                else
                {
                  DWORD2(v63) = 0;
                  v42 = (__int64 *)v42[2];
                }
              }
              if ( *((_BYTE *)v43 + 25) || v13 < *((_DWORD *)v43 + 8) )
              {
                if ( v38[1] == 0x492492492492492LL )
                  sub_14014F360(v43, 0);
                v74 = v38;
                v75 = 0;
                v44 = sub_146E8BA20(56);
                v82 = v44 + 32;
                *(_DWORD *)(v44 + 32) = v13;
                *(_QWORD *)(v44 + 40) = v40;
                *(_QWORD *)(v44 + 48) = v39;
                v69 = 0;
                *(_QWORD *)v44 = v41;
                *(_QWORD *)(v44 + 8) = v41;
                *(_QWORD *)(v44 + 16) = v41;
                *(_WORD *)(v44 + 24) = 0;
                v75 = 0;
                v90 = v63;
                sub_14014F0E0(v38, &v90, v44);
                v39 = (volatile signed __int32 *)*((_QWORD *)&v69 + 1);
              }
              v57 = v59 & 0xFFFFFFF7;
              if ( v39 )
              {
                if ( _InterlockedExchangeAdd(v39 + 2, 0xFFFFFFFF) == 1 )
                {
                  v45 = *((_QWORD *)&v69 + 1);
                  (***((void (__fastcall ****)(_QWORD))&v69 + 1))(*((_QWORD *)&v69 + 1));
                  if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v45 + 12), 0xFFFFFFFF) == 1 )
                    (*(void (__fastcall **)(_QWORD))(**((_QWORD **)&v69 + 1) + 8LL))(*((_QWORD *)&v69 + 1));
                }
              }
              ++v9;
              v46 = v77;
              if ( v77 )
              {
                if ( _InterlockedExchangeAdd(v77 + 2, 0xFFFFFFFF) == 1 )
                {
                  (**(void (__fastcall ***)(volatile signed __int32 *))v46)(v46);
                  if ( _InterlockedExchangeAdd(v46 + 3, 0xFFFFFFFF) == 1 )
                    (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v46 + 8LL))(v46);
                }
              }
              v47 = v65;
              if ( v65 )
              {
                if ( _InterlockedExchangeAdd(v65 + 2, 0xFFFFFFFF) == 1 )
                {
                  (**(void (__fastcall ***)(volatile signed __int32 *))v47)(v47);
                  if ( _InterlockedExchangeAdd(v47 + 3, 0xFFFFFFFF) == 1 )
                    (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v47 + 8LL))(v47);
                }
              }
              if ( v93 >= 8 )
              {
                v48 = 2 * v93 + 2;
                v49 = v91[0];
                if ( v48 >= 0x1000 )
                {
                  v48 = 2 * v93 + 41;
                  v49 = *(_QWORD *)(v91[0] - 8LL);
                  if ( (unsigned __int64)(v91[0] - v49 - 8) > 0x1F )
                    sub_148AAF304(v49, v48);
                }
                sub_146E9F3A0(v49, v48);
              }
              v92 = 0;
              v93 = 7;
              LOWORD(v91[0]) = 0;
              v3 = a1;
              v6 = v62;
            }
            v2 = v70;
          }
        }
      }
      v4 = v60 + 1;
      v60 = v4;
      if ( v4 >= v6 )
        break;
      LODWORD(v8) = v78;
    }
  }
  v50 = v3[262];
  v51 = sub_146E8C7D0(&unk_149DE65A0);
  v52 = sub_146EC8E30(v50, &v70, v51);
  v53 = sub_1401E9C70(&v78, v52);
  result = sub_1401E5080(v3 + 283, v53);
  v55 = v79;
  if ( v79 )
  {
    result = (unsigned int)_InterlockedExchangeAdd(v79 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v55)(v55);
      result = (unsigned int)_InterlockedExchangeAdd(v55 + 3, 0xFFFFFFFF);
      if ( (_DWORD)result == 1 )
        result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v55 + 8LL))(v55);
    }
  }
  v56 = v71;
  if ( v71 )
  {
    result = (unsigned int)_InterlockedExchangeAdd(v71 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v56)(v56);
      if ( _InterlockedExchangeAdd(v56 + 3, 0xFFFFFFFF) == 1 )
        return (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v56 + 8LL))(v56);
    }
  }
  return result;
}

