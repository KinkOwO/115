// holder_sub_1447E7440_0x1447e7440

__int64 __fastcall sub_1447E7440(
        __int64 a1,
        int a2,
        int a3,
        int a4,
        __int64 a5,
        int a6,
        __int64 a7,
        unsigned int a8,
        __int16 a9,
        int a10,
        int a11)
{
  int v11; // r12d
  int v12; // r13d
  int v13; // esi
  _QWORD *v15; // r14
  __int64 v16; // rax
  __int64 (__fastcall ***v17)(_QWORD); // rcx
  __int64 result; // rax
  __int64 v19; // rbx
  __int64 v20; // rax
  __int64 v21; // rdx
  __int64 v22; // rcx
  __int64 v23; // r8
  int v24; // r13d
  __int64 *v25; // rcx
  __int64 *v26; // rsi
  int *v27; // rbx
  int v28; // r12d
  int v29; // eax
  int v30; // ebx
  __int64 v31; // rax
  __int64 v32; // rsi
  int *v33; // r8
  _QWORD *v34; // rdx
  __int64 v35; // rax
  __int64 v36; // rcx
  __int64 v37; // rax
  float v38; // xmm1_4
  __int64 v39; // rdx
  __int64 v40; // rcx
  __int64 v41; // r8
  __int64 v42; // rax
  __int64 v43; // rax
  int v44; // r15d
  int v45; // eax
  int v46; // eax
  int v47; // r13d
  int *v48; // rbx
  int v49; // r12d
  int v50; // ebx
  __int64 v51; // rax
  _DWORD *v52; // rsi
  _QWORD *v53; // r12
  _QWORD *i; // rdi
  _DWORD *v55; // rbx
  _DWORD *v56; // rbx
  int v57; // eax
  int v58; // eax
  __int64 v59; // rax
  __int64 v60; // rbx
  __int64 v61; // rax
  int v62; // eax
  int v63; // ebx
  int v64; // eax
  int v65; // eax
  _QWORD *v66; // r12
  _QWORD *j; // rdi
  _DWORD *v68; // rbx
  _DWORD *v69; // rbx
  int v70; // eax
  int v71; // eax
  _QWORD *v72; // rdx
  __int64 v73; // [rsp+40h] [rbp-71h]
  int v74; // [rsp+68h] [rbp-49h] BYREF
  __int64 v75; // [rsp+70h] [rbp-41h] BYREF
  _DWORD *v76; // [rsp+78h] [rbp-39h] BYREF
  int v77; // [rsp+80h] [rbp-31h] BYREF
  _QWORD v78[2]; // [rsp+88h] [rbp-29h] BYREF
  int v79; // [rsp+98h] [rbp-19h] BYREF
  _DWORD *v80; // [rsp+A0h] [rbp-11h]
  char v81[8]; // [rsp+A8h] [rbp-9h] BYREF
  __int64 v82; // [rsp+B0h] [rbp-1h]
  int v83; // [rsp+100h] [rbp+4Fh]

  v83 = a2;
  v82 = -2;
  v11 = a4;
  v12 = a3;
  v13 = a2;
  v15 = (_QWORD *)qword_14E63AE60;
  if ( qword_14E63AE60
    || ((v16 = sub_146E8BA20(336), (v76 = (_DWORD *)v16) == 0)
      ? (v17 = 0)
      : (v17 = (__int64 (__fastcall ***)(_QWORD))sub_1447E41D0(v16)),
        qword_14E63AE60 = (__int64)v17,
        result = (**v17)(v17),
        (v15 = (_QWORD *)qword_14E63AE60) != 0) )
  {
    result = sub_1447EDF00(v15, a1);
    if ( (_BYTE)result )
    {
      v19 = a5;
      if ( a5 )
      {
        sub_140EE99F0(v81, 0);
        if ( !qword_14E66C090 )
          return sub_140EE9AD0(v81);
        v20 = sub_1401DF330();
        if ( !sub_140EEB630(v20) )
          return sub_140EE9AD0(v81);
        v77 = a6 & 1;
        if ( (a6 & 1) == 0 || (*(_DWORD *)(a1 + 348) & 0x211) != 0x211 || dword_14DC668F8 <= 0 )
        {
LABEL_40:
          v38 = (float)dword_14DC668F4;
          if ( (a6 & 3) != 0 )
            v38 = (float)dword_14DC668F0;
          if ( v38 <= 0.0 )
          {
            if ( (__int64)abs64(v19) < sub_1447EAE30(a8) )
              return sub_140EE9AD0(v81);
            v42 = sub_1401DCCB0(v40, v39, v41);
            if ( !(unsigned __int16)sub_1403F5B80(v42, 204) )
              return sub_140EE9AD0(v81);
          }
          v43 = sub_1401DCCB0(v22, v21, v23);
          if ( (unsigned __int16)sub_1403F5B80(v43, 204) && !dword_14DC668F0 && dword_14DC668F8 > 0 && v77 )
          {
            v44 = a6 | 0x100;
            v45 = sub_1401DF330();
            sub_140EEACE0(v45, v13, 0, 1, 0, 0);
            v46 = sub_1401DF330();
            LOBYTE(v73) = 0;
            sub_140EEAF90(v46, v12, v11, 0, 1065353216, 1, 0, v73);
            v47 = sub_145B8C7D0(a1);
            v77 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 832LL))(a1);
            v48 = &v77;
            if ( v77 > 150 )
              v48 = (int *)&unk_14A3F3010;
            v49 = sub_145B8C890(a1) - *v48;
            v50 = sub_145B8C8C0(a1);
            if ( (v44 & dword_14E665E80) != 0 )
            {
              v47 += qword_14E665E90;
              v49 += HIDWORD(qword_14E665E90);
              v50 += dword_14E665E98;
            }
            if ( *(_BYTE *)(a1 + 2032) && (v44 & *(_DWORD *)(a1 + 2036)) != 0 )
            {
              v47 = *(_DWORD *)(a1 + 2020);
              v49 = *(_DWORD *)(a1 + 2024);
              v50 = *(_DWORD *)(a1 + 2028);
            }
            LODWORD(v75) = -1;
            v74 = 0;
            if ( (unsigned __int8)sub_1447E6550((_DWORD)v15, v13, a3, a4, (__int64)&v75, (__int64)&v74) )
              return sub_140EE9AD0(v81);
            v51 = sub_146E8BA20(408);
            v78[0] = v51;
            if ( v51 )
              v52 = (_DWORD *)sub_1447E40D0(v51);
            else
              v52 = 0;
            v76 = v52;
            sub_1447EBED0((_DWORD)v52, v47, v49, v50, a5, a6 | 0x100, 10000, 10000, 0, a8, a9);
            v52[87] = a10;
            v52[88] = a11;
            sub_1447EF6E0(v52);
            if ( (_DWORD)v75 != -1 )
            {
              v52[85] = v75;
              v52[86] = v74;
            }
            v53 = (_QWORD *)v15[10];
            for ( i = (_QWORD *)v15[9]; i != v53; ++i )
            {
              v55 = (_DWORD *)*i;
              if ( (unsigned __int8)sub_146E92C40(*i + 384LL, v52 + 96) && v55[20] == v52[20] )
              {
                v56 = (_DWORD *)*i;
                v57 = sub_146E9F840(*i + 304LL);
                v56[42] = v57;
                v56[43] = v57 + 400;
                v58 = v56[20];
                v56[44] = v58;
                v56[45] = v58 - 32;
              }
            }
          }
          else
          {
            if ( (a6 & dword_14E665E80) != 0 )
            {
              v13 += qword_14E665E90;
              v83 = v13;
              v12 += HIDWORD(qword_14E665E90);
              v11 += dword_14E665E98;
            }
            if ( *(_BYTE *)(a1 + 2032) && (a6 & *(_DWORD *)(a1 + 2036)) != 0 )
            {
              v13 = *(_DWORD *)(a1 + 2020);
              v83 = v13;
              v12 = *(_DWORD *)(a1 + 2024);
              v11 = *(_DWORD *)(a1 + 2028);
            }
            if ( (*(_DWORD *)(a1 + 348) & 0x211) == 0x211 )
            {
              v59 = sub_1450BE2E0(a1);
              v60 = v59;
              if ( v59 )
              {
                v13 += sub_145D6DA50(v59, 0);
                v83 = v13;
                v12 += sub_145D6DA50(v60, 1);
                v11 += sub_145D6DA50(v60, 2);
              }
            }
            LODWORD(v75) = -1;
            v74 = 0;
            if ( (unsigned __int8)sub_1447E6550((_DWORD)v15, v13, v12, v11, (__int64)&v75, (__int64)&v74) )
              return sub_140EE9AD0(v81);
            v61 = sub_146E8BA20(408);
            v78[0] = v61;
            if ( v61 )
              v52 = (_DWORD *)sub_1447E40D0(v61);
            else
              v52 = 0;
            v76 = v52;
            v62 = sub_1401DF330();
            v63 = sub_140EEACE0(v62, v83, 0, 1, 0, 0);
            v64 = sub_1401DF330();
            LOBYTE(v73) = 0;
            v65 = sub_140EEAF90(v64, v12, v11, 0, 1065353216, 1, 0, v73);
            sub_1447EBED0((_DWORD)v52, v83, v12, v11, a5, a6, v63, v65, a7, a8, a9);
            v52[87] = a10;
            v52[88] = a11;
            sub_1447EF6E0(v52);
            if ( (_DWORD)v75 != -1 )
            {
              v52[85] = v75;
              v52[86] = v74;
            }
            v66 = (_QWORD *)v15[10];
            for ( j = (_QWORD *)v15[9]; j != v66; ++j )
            {
              v68 = (_DWORD *)*j;
              if ( (unsigned __int8)sub_146E92C40(*j + 384LL, v52 + 96) && v68[20] == v52[20] )
              {
                v69 = (_DWORD *)*j;
                v70 = sub_146E9F840(*j + 304LL);
                v69[42] = v70;
                v69[43] = v70 + 400;
                v71 = v69[20];
                v69[44] = v71;
                v69[45] = v71 - 32;
              }
            }
          }
          v72 = (_QWORD *)v15[10];
          if ( v72 == (_QWORD *)v15[11] )
          {
            sub_140295D90(v15 + 9, v72, &v76);
          }
          else
          {
            *v72 = v52;
            v15[10] += 8LL;
          }
          return sub_140EE9AD0(v81);
        }
        v24 = sub_145B8C670(a1);
        v25 = *(__int64 **)(v15[25] + 8LL);
        v26 = (__int64 *)v15[25];
        while ( !*((_BYTE *)v25 + 25) )
        {
          if ( *((_DWORD *)v25 + 8) >= v24 )
          {
            v26 = v25;
            v25 = (__int64 *)*v25;
          }
          else
          {
            v25 = (__int64 *)v25[2];
          }
        }
        if ( *((_BYTE *)v26 + 25) || v24 < *((_DWORD *)v26 + 8) )
          v26 = (__int64 *)v15[25];
        LODWORD(v75) = sub_145B8C7D0(a1);
        v74 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 832LL))(a1);
        v27 = &v74;
        if ( v74 > 150 )
          v27 = (int *)&unk_14A3F3010;
        v28 = sub_145B8C890(a1) - *v27;
        v29 = sub_145B8C8C0(a1);
        v74 = v29;
        v30 = v75;
        if ( (a6 & dword_14E665E80) != 0 )
        {
          v30 = qword_14E665E90 + v75;
          v28 += HIDWORD(qword_14E665E90);
          v74 = dword_14E665E98 + v29;
        }
        if ( *(_BYTE *)(a1 + 2032) && (a6 & *(_DWORD *)(a1 + 2036)) != 0 )
        {
          v30 = *(_DWORD *)(a1 + 2020);
          v28 = *(_DWORD *)(a1 + 2024);
          v74 = *(_DWORD *)(a1 + 2028);
        }
        if ( v26 == (__int64 *)v15[25] )
        {
          v31 = sub_146E8BA20(400);
          v76 = (_DWORD *)v31;
          if ( v31 )
            v32 = sub_1447E3ED0(v31);
          else
            v32 = 0;
          sub_1447EB510(v32, v30, v28, v74, a5, a7, 0, -1);
          LODWORD(v78[0]) = v24;
          v78[1] = v32;
          v33 = (int *)v78;
          v34 = &v76;
        }
        else
        {
          v76 = (_DWORD *)v26[5];
          if ( !v76 )
          {
LABEL_39:
            v11 = a4;
            v12 = a3;
            v13 = v83;
            v19 = a5;
            goto LABEL_40;
          }
          v35 = sub_146E8BA20(400);
          v78[0] = v35;
          if ( v35 )
            v36 = sub_1447E3ED0(v35);
          else
            v36 = 0;
          v75 = v36;
          v37 = (__int64)v76;
          *((_BYTE *)v76 + 220) = 1;
          sub_1447EB510(v36, v30, v28, v74, a5, a7, v37, -1);
          sub_140B80220(v15 + 25, v78, v26);
          v79 = v24;
          v80 = (_DWORD *)v75;
          sub_1413A2960(v15 + 25, v78, &v79);
          v79 = -1;
          v80 = v76;
          v33 = &v79;
          v34 = v78;
        }
        sub_1413A2960(v15 + 25, v34, v33);
        goto LABEL_39;
      }
    }
  }
  return result;
}

