// reader_sub_1444E5FD0

void __fastcall sub_1444E5FD0(_QWORD *a1, int a2, unsigned int a3, unsigned __int16 *a4, __int64 a5)
{
  unsigned __int16 *v5; // r15
  unsigned __int64 v7; // rsi
  _QWORD *v8; // r14
  __int64 v9; // rdi
  __int64 v10; // rcx
  __int64 v11; // rax
  void (__fastcall ***v12)(_QWORD); // rcx
  __int64 v13; // rax
  __int64 v14; // rcx
  __int64 v15; // rbx
  __int64 v16; // rax
  unsigned __int16 *v17; // rdx
  __int64 v18; // rcx
  int v19; // r8d
  __int64 v20; // r13
  int v21; // r9d
  int v22; // ecx
  int v23; // r9d
  int v24; // ecx
  int v25; // r9d
  __int64 v26; // kr00_8
  __int64 v27; // rdx
  __int16 *v28; // r12
  unsigned int *v29; // r15
  __int64 *v30; // rbx
  _WORD *v31; // r14
  __int64 v32; // rsi
  __int64 v33; // rcx
  __int64 v34; // rdx
  __int64 v35; // rdx
  __int64 v36; // rdx
  unsigned int v37; // esi
  __int64 v38; // rcx
  __int64 v39; // rax
  void (__fastcall ***v40)(_QWORD); // rcx
  __int64 v41; // rdx
  __int64 v42; // rdx
  __int64 v43; // rbx
  __int64 v44; // rcx
  __int64 v45; // r9
  _BYTE *v46; // rcx
  __int64 v47; // rcx
  __int64 v48; // rcx
  unsigned __int64 v49; // rdx
  __int16 v50; // [rsp+20h] [rbp-E0h] BYREF
  bool v51; // [rsp+22h] [rbp-DEh]
  __int64 v52; // [rsp+28h] [rbp-D8h]
  __int128 v53; // [rsp+30h] [rbp-D0h] BYREF
  __int128 v54; // [rsp+40h] [rbp-C0h] BYREF
  __int64 v55; // [rsp+50h] [rbp-B0h]
  __int64 v56; // [rsp+58h] [rbp-A8h]
  _QWORD *v57; // [rsp+60h] [rbp-A0h]
  unsigned __int16 *v58; // [rsp+68h] [rbp-98h]
  __int64 v59; // [rsp+70h] [rbp-90h]
  __int64 v60; // [rsp+78h] [rbp-88h]
  __int128 *v61; // [rsp+80h] [rbp-80h]
  _BYTE v62[288]; // [rsp+90h] [rbp-70h] BYREF
  _BYTE v63[1968]; // [rsp+1B0h] [rbp+B0h] BYREF
  _BYTE v64[1968]; // [rsp+960h] [rbp+860h] BYREF
  __int64 v65; // [rsp+1110h] [rbp+1010h] BYREF
  int v66; // [rsp+1118h] [rbp+1018h]

  v59 = -2;
  v5 = a4;
  v58 = a4;
  v7 = a2;
  v8 = a1;
  v57 = a1;
  v9 = a5;
  v56 = a5;
  v10 = qword_14E664BF8;
  if ( !qword_14E664BF8 )
  {
    v11 = sub_146E8BA20(2792);
    v52 = v11;
    if ( v11 )
      v12 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v11);
    else
      v12 = 0;
    qword_14E664BF8 = (__int64)v12;
    (**v12)(v12);
    v10 = qword_14E664BF8;
  }
  sub_1444D2D60(v10, &v54);
  if ( v7 < (__int64)(*((_QWORD *)&v54 + 1) - v54) >> 3 && *(_DWORD *)(v54 + 8 * v7) == a3 )
  {
    if ( *(_DWORD *)(v54 + 8 * v7 + 4) )
    {
      v13 = sub_1444D2BB0(v54);
      v15 = sub_1444D2B60(v13, a3);
      if ( v15 )
      {
        v16 = sub_1444D2BB0(v14);
        v17 = (unsigned __int16 *)sub_1444D2A20(v16);
        if ( v17 )
        {
          if ( v5 )
          {
            v50 = 0;
            v51 = 0;
            v65 = 0;
            v66 = 0;
            v19 = *(_DWORD *)(v15 + 8);
            v20 = 2;
            if ( v19 != 2 )
              goto LABEL_19;
            v21 = v17[447];
            v22 = v5[447];
            if ( v17[447] )
            {
              LOBYTE(v50) = v22 == 0;
              LODWORD(v65) = v22 - v21;
            }
            v23 = v17[626];
            v24 = v5[626];
            if ( v17[626] )
            {
              HIBYTE(v50) = v24 == 0;
              HIDWORD(v65) = v24 - v23;
            }
            v25 = v17[805];
            v18 = v5[805];
            if ( v17[805] )
            {
              v51 = (_DWORD)v18 == 0;
              v18 = (unsigned int)(v18 - v25);
              v66 = v18;
              v26 = 0;
            }
            else
            {
LABEL_19:
              v26 = (unsigned int)(v19 - 2);
            }
            switch ( v26 )
            {
              case 0LL:
                v28 = &v50;
                v29 = (unsigned int *)&v65;
                v30 = v8 + 347;
                v31 = v17 + 447;
                v32 = 3;
                v52 = 3;
                do
                {
                  if ( *v31 )
                  {
                    v33 = v30[6];
                    if ( v33 )
                    {
                      v34 = 1;
                      if ( !*(_BYTE *)v28 )
                        v34 = 2;
                      sub_146AF0950(v33, v34);
                      (*(void (__fastcall **)(__int64))(*(_QWORD *)v30[6] + 432LL))(v30[6]);
                    }
                    if ( *v30 && !(unsigned __int8)sub_141FB6530(*v30) )
                    {
                      sub_146AF0950(*v30, 0);
                      (*(void (__fastcall **)(__int64))(*(_QWORD *)*v30 + 432LL))(*v30);
                      LOBYTE(v35) = 1;
                      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)*v30 + 16LL))(*v30, v35);
                    }
                    v18 = v30[12];
                    if ( v18 && !(unsigned __int8)sub_141FB6530(v18) )
                    {
                      sub_146AF0950(v30[12], 0);
                      (*(void (__fastcall **)(__int64))(*(_QWORD *)v30[12] + 432LL))(v30[12]);
                      LOBYTE(v36) = 1;
                      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v30[12] + 16LL))(v30[12], v36);
                    }
                    if ( v30[90] )
                    {
                      v37 = *v29;
                      if ( *v29 )
                      {
                        v38 = qword_14E664BF8;
                        if ( !qword_14E664BF8 )
                        {
                          v39 = sub_146E8BA20(2792);
                          v60 = v39;
                          if ( v39 )
                            v40 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v39);
                          else
                            v40 = 0;
                          qword_14E664BF8 = (__int64)v40;
                          (**v40)(v40);
                          v38 = qword_14E664BF8;
                        }
                        v61 = &v53;
                        v53 = 0;
                        v41 = v30[91];
                        if ( v41 )
                        {
                          _InterlockedIncrement((volatile signed __int32 *)(v41 + 8));
                          v41 = v30[91];
                        }
                        *(_QWORD *)&v53 = v30[90];
                        *((_QWORD *)&v53 + 1) = v41;
                        sub_1444D3910(v38, &v53, v37);
                        (*(void (__fastcall **)(__int64))(*(_QWORD *)v30[90] + 432LL))(v30[90]);
                        LOBYTE(v42) = 1;
                        (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v30[90] + 16LL))(v30[90], v42);
                      }
                      v32 = v52;
                    }
                  }
                  v31 += 179;
                  v28 = (__int16 *)((char *)v28 + 1);
                  ++v29;
                  v30 += 2;
                  v52 = --v32;
                }
                while ( v32 );
                v9 = v56;
                v8 = v57;
                v5 = v58;
                break;
              case 1LL:
                v18 = v8[487];
                if ( v18 )
                {
                  v27 = 1;
                  goto LABEL_50;
                }
                break;
              case 2LL:
              case 3LL:
              case 4LL:
              case 5LL:
              case 19LL:
                v18 = v8[487];
                if ( v18 )
                {
                  v27 = 2;
LABEL_50:
                  sub_146AF0950(v18, v27);
                  (*(void (__fastcall **)(_QWORD))(*(_QWORD *)v8[487] + 432LL))(v8[487]);
                  (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)v8[487] + 16LL))(v8[487], 1);
                }
                break;
              default:
                break;
            }
            v43 = sub_1444D2BB0(v18);
            sub_148AA1E60(v63, v5, 1968);
            sub_1444D38A0(v43, v63);
            v45 = sub_1444D2BB0(v44);
            v46 = v62;
            do
            {
              *(_OWORD *)v46 = *(_OWORD *)v9;
              *((_OWORD *)v46 + 1) = *(_OWORD *)(v9 + 16);
              *((_OWORD *)v46 + 2) = *(_OWORD *)(v9 + 32);
              *((_OWORD *)v46 + 3) = *(_OWORD *)(v9 + 48);
              *((_OWORD *)v46 + 4) = *(_OWORD *)(v9 + 64);
              *((_OWORD *)v46 + 5) = *(_OWORD *)(v9 + 80);
              *((_OWORD *)v46 + 6) = *(_OWORD *)(v9 + 96);
              v46 += 128;
              *((_OWORD *)v46 - 1) = *(_OWORD *)(v9 + 112);
              v9 += 128;
              --v20;
            }
            while ( v20 );
            *(_OWORD *)v46 = *(_OWORD *)v9;
            *((_QWORD *)v46 + 2) = *(_QWORD *)(v9 + 16);
            *((_DWORD *)v46 + 6) = *(_DWORD *)(v9 + 24);
            sub_1444D40D0(v45, v62);
            sub_148AA1E60(v64, v5, 1968);
            sub_1444E0750(v8, v64);
            sub_1444DD420(v8, 0, 0xFFFFFFFFLL);
            v47 = v8[323];
            if ( v47 )
              sub_143FD16D0(v47, *v5);
          }
        }
      }
    }
  }
  v48 = v54;
  if ( (_QWORD)v54 )
  {
    v49 = (v55 - v54) & 0xFFFFFFFFFFFFFFF8uLL;
    if ( v49 >= 0x1000 )
    {
      v49 += 39LL;
      v48 = *(_QWORD *)(v54 - 8);
      if ( (unsigned __int64)(v54 - v48 - 8) > 0x1F )
        sub_148AAF304(v48, v49);
    }
    sub_146E9F3A0(v48, v49);
    v54 = 0;
    v55 = 0;
  }
}

