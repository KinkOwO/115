// ins_sub_140766270

void __fastcall sub_140766270(__int64 a1, __int64 a2)
{
  __int64 v2; // rsi
  __int64 v3; // r15
  __int64 v4; // rcx
  __int64 v5; // rdx
  char *v6; // rbx
  char *v7; // rdi
  __int64 v8; // rcx
  unsigned __int64 v9; // rdx
  __int64 v10; // r8
  __int64 v11; // rcx
  __int64 v12; // rcx
  unsigned __int64 v13; // rdx
  __int64 v14; // r8
  __int64 v15; // rcx
  __int64 v16; // rcx
  unsigned __int64 v17; // rdx
  __int64 v18; // r8
  __int64 v19; // rcx
  __int64 v20; // rcx
  unsigned __int64 v21; // rdx
  __int64 v22; // r8
  __int64 v23; // rcx
  char *v24; // rax
  __int64 v25; // r8
  unsigned __int64 v26; // r14
  _QWORD *v27; // rdi
  _QWORD *v28; // rbx
  __int64 v29; // rsi
  __int64 v30; // rbx
  __int64 v31; // rcx
  __int64 v32; // rax
  void (__fastcall ***v33)(_QWORD); // rcx
  __int64 v34; // r15
  int v35; // r12d
  int v36; // edi
  int v37; // r13d
  int v38; // r8d
  int v39; // ecx
  int v40; // eax
  _QWORD *v41; // rdx
  __int64 v42; // rcx
  unsigned __int64 v43; // rdx
  __int64 v44; // r8
  __int64 v45; // rcx
  __int64 v46; // rcx
  unsigned __int64 v47; // rdx
  __int64 v48; // r8
  __int64 v49; // rcx
  __int64 v50; // rcx
  unsigned __int64 v51; // rdx
  __int64 v52; // r8
  __int64 v53; // rcx
  __int64 v54; // rcx
  unsigned __int64 v55; // rdx
  __int64 v56; // r8
  __int64 v57; // rcx
  __int64 v58; // r8
  __int64 v59; // rax
  __int64 v60; // rcx
  char *v61; // rdi
  __int64 v62; // rcx
  unsigned __int64 v63; // rdx
  __int64 v64; // r8
  __int64 v65; // rcx
  __int64 v66; // rcx
  unsigned __int64 v67; // rdx
  __int64 v68; // r8
  __int64 v69; // rcx
  __int64 v70; // rcx
  unsigned __int64 v71; // rdx
  __int64 v72; // r8
  __int64 v73; // rcx
  __int64 v74; // rcx
  __int64 v75; // r8
  char *v76; // rax
  __int128 v77; // [rsp+60h] [rbp-49h] BYREF
  __int64 v78; // [rsp+70h] [rbp-39h]
  __int128 v79; // [rsp+78h] [rbp-31h] BYREF
  __int128 v80; // [rsp+88h] [rbp-21h] BYREF
  __int64 v81; // [rsp+98h] [rbp-11h]
  __int64 v82; // [rsp+A0h] [rbp-9h]
  __int64 v83; // [rsp+A8h] [rbp-1h]
  __int128 *v84; // [rsp+B0h] [rbp+7h]
  void *retaddr; // [rsp+108h] [rbp+5Fh]
  int v88; // [rsp+120h] [rbp+77h] BYREF
  __int64 v89; // [rsp+128h] [rbp+7Fh] BYREF

  v82 = -2;
  v2 = a2;
  v3 = a1;
  v4 = *(_QWORD *)(a1 + 6960);
  if ( v4 && a2 )
  {
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v4 + 16LL))(v4, 0);
    v77 = 0;
    v78 = 0;
    if ( (unsigned __int8)sub_14087A490(v2, &v77) )
    {
      if ( (_QWORD)v77 == *((_QWORD *)&v77 + 1) )
      {
        v6 = (char *)v77;
        if ( !(_QWORD)v77 )
          return;
        v7 = (char *)*((_QWORD *)&v77 + 1);
        if ( (_QWORD)v77 != *((_QWORD *)&v77 + 1) )
        {
          do
          {
            v8 = *((_QWORD *)v6 + 15);
            if ( v8 )
            {
              v9 = 4 * ((*((_QWORD *)v6 + 17) - v8) >> 2);
              if ( v9 >= 0x1000 )
              {
                v9 += 39LL;
                v10 = *(_QWORD *)(v8 - 8);
                v11 = v8 - v10;
                if ( (unsigned __int64)(v11 - 8) > 0x1F )
                  sub_148AAF304(v11, v9);
                v8 = v10;
              }
              sub_146E9F3A0(v8, v9);
              *((_QWORD *)v6 + 15) = 0;
              *((_QWORD *)v6 + 16) = 0;
              *((_QWORD *)v6 + 17) = 0;
            }
            v12 = *((_QWORD *)v6 + 11);
            if ( v12 )
            {
              v13 = 4 * ((*((_QWORD *)v6 + 13) - v12) >> 2);
              if ( v13 >= 0x1000 )
              {
                v13 += 39LL;
                v14 = *(_QWORD *)(v12 - 8);
                v15 = v12 - v14;
                if ( (unsigned __int64)(v15 - 8) > 0x1F )
                  sub_148AAF304(v15, v13);
                v12 = v14;
              }
              sub_146E9F3A0(v12, v13);
              *((_QWORD *)v6 + 11) = 0;
              *((_QWORD *)v6 + 12) = 0;
              *((_QWORD *)v6 + 13) = 0;
            }
            v16 = *((_QWORD *)v6 + 6);
            if ( v16 )
            {
              v17 = 8 * ((*((_QWORD *)v6 + 8) - v16) >> 3);
              if ( v17 >= 0x1000 )
              {
                v17 += 39LL;
                v18 = *(_QWORD *)(v16 - 8);
                v19 = v16 - v18;
                if ( (unsigned __int64)(v19 - 8) > 0x1F )
                  sub_148AAF304(v19, v17);
                v16 = v18;
              }
              sub_146E9F3A0(v16, v17);
              *((_QWORD *)v6 + 6) = 0;
              *((_QWORD *)v6 + 7) = 0;
              *((_QWORD *)v6 + 8) = 0;
            }
            v20 = *((_QWORD *)v6 + 3);
            if ( v20 )
            {
              v21 = (*((_QWORD *)v6 + 5) - v20) & 0xFFFFFFFFFFFFFFF8uLL;
              if ( v21 >= 0x1000 )
              {
                v21 += 39LL;
                v22 = *(_QWORD *)(v20 - 8);
                v23 = v20 - v22;
                if ( (unsigned __int64)(v23 - 8) > 0x1F )
                  goto LABEL_107;
                v20 = v22;
              }
              sub_146E9F3A0(v20, v21);
              *((_QWORD *)v6 + 3) = 0;
              *((_QWORD *)v6 + 4) = 0;
              *((_QWORD *)v6 + 5) = 0;
            }
            v6 += 144;
          }
          while ( v6 != v7 );
          v6 = (char *)v77;
        }
        v23 = v78 - (_QWORD)v6;
        v21 = 144 * ((v78 - (__int64)v6) / 144);
        v24 = v6;
        if ( v21 < 0x1000 )
          goto LABEL_31;
        v21 += 39LL;
        v6 = (char *)*((_QWORD *)v6 - 1);
        if ( (unsigned __int64)(v24 - v6 - 8) <= 0x1F )
          goto LABEL_31;
        goto LABEL_107;
      }
      LOBYTE(v5) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v3 + 6960) + 16LL))(*(_QWORD *)(v3 + 6960), v5);
      sub_146F1E010(*(_QWORD *)(v3 + 6960));
      sub_142268950(*(_QWORD *)(v3 + 6960), 0xFFFFFFFFLL);
      v26 = 0;
      v27 = (_QWORD *)*((_QWORD *)&v77 + 1);
      v28 = (_QWORD *)v77;
      if ( (*((_QWORD *)&v77 + 1) - (_QWORD)v77) / 144LL )
      {
        v29 = 0;
        do
        {
          LOBYTE(v25) = 1;
          v30 = sub_140283D60(qword_14E683B38, LODWORD(v28[v29]), v25);
          if ( v30 )
          {
            v31 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v32 = sub_146E8BA20(1472);
              v83 = v32;
              if ( v32 )
                v33 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v32);
              else
                v33 = 0;
              qword_14E638F28 = (__int64)v33;
              (**v33)(v33);
              v31 = qword_14E638F28;
            }
            if ( sub_1444EC6B0(v31, v30) )
            {
              v34 = *(_QWORD *)(v3 + 6960);
              v84 = &v80;
              v80 = 0;
              v81 = 0;
              *(_QWORD *)&v79 = &v89;
              v89 = 0;
              v35 = dword_14F1C0920;
              v36 = dword_14F1C091C;
              v37 = dword_14F1C0918;
              sub_146E920A0(v30, &v88);
              v38 = v88;
              v39 = *(_DWORD *)v30 + v88 + 196;
              v40 = *(_DWORD *)(v30 + 4);
              if ( v40 && v39 && v40 != v39 && retaddr )
              {
                sub_146D89B40(retaddr, v30);
                v38 = v88;
              }
              v41 = (_QWORD *)(v30 + 496);
              if ( *(_QWORD *)(v30 + 520) >= 8u )
                v41 = (_QWORD *)*v41;
              sub_146F1AD60(v34, (_DWORD)v41, v38, 0, v37, v36, v36, v35, (__int64)&v89, 1, 1, (__int64)&v80);
              v3 = a1;
            }
          }
          ++v26;
          v29 += 18;
          v27 = (_QWORD *)*((_QWORD *)&v77 + 1);
          v28 = (_QWORD *)v77;
        }
        while ( v26 < (*((_QWORD *)&v77 + 1) - (_QWORD)v77) / 144LL );
        v2 = a2;
      }
      if ( v28 != v27 )
      {
        do
        {
          v42 = v28[15];
          if ( v42 )
          {
            v43 = 4 * ((v28[17] - v42) >> 2);
            if ( v43 >= 0x1000 )
            {
              v43 += 39LL;
              v44 = *(_QWORD *)(v42 - 8);
              v45 = v42 - v44;
              if ( (unsigned __int64)(v45 - 8) > 0x1F )
                sub_148AAF304(v45, v43);
              v42 = v44;
            }
            sub_146E9F3A0(v42, v43);
            v28[15] = 0;
            v28[16] = 0;
            v28[17] = 0;
          }
          v46 = v28[11];
          if ( v46 )
          {
            v47 = 4 * ((v28[13] - v46) >> 2);
            if ( v47 >= 0x1000 )
            {
              v47 += 39LL;
              v48 = *(_QWORD *)(v46 - 8);
              v49 = v46 - v48;
              if ( (unsigned __int64)(v49 - 8) > 0x1F )
                sub_148AAF304(v49, v47);
              v46 = v48;
            }
            sub_146E9F3A0(v46, v47);
            v28[11] = 0;
            v28[12] = 0;
            v28[13] = 0;
          }
          v50 = v28[6];
          if ( v50 )
          {
            v51 = 8 * ((v28[8] - v50) >> 3);
            if ( v51 >= 0x1000 )
            {
              v51 += 39LL;
              v52 = *(_QWORD *)(v50 - 8);
              v53 = v50 - v52;
              if ( (unsigned __int64)(v53 - 8) > 0x1F )
                sub_148AAF304(v53, v51);
              v50 = v52;
            }
            sub_146E9F3A0(v50, v51);
            v28[6] = 0;
            v28[7] = 0;
            v28[8] = 0;
          }
          v54 = v28[3];
          if ( v54 )
          {
            v55 = 8 * ((v28[5] - v54) >> 3);
            if ( v55 >= 0x1000 )
            {
              v55 += 39LL;
              v56 = *(_QWORD *)(v54 - 8);
              v57 = v54 - v56;
              if ( (unsigned __int64)(v57 - 8) > 0x1F )
                sub_148AAF304(v57, v55);
              v54 = v56;
            }
            sub_146E9F3A0(v54, v55);
            v28[3] = 0;
            v28[4] = 0;
            v28[5] = 0;
          }
          v28 += 18;
        }
        while ( v28 != v27 );
        v28 = (_QWORD *)v77;
      }
      *((_QWORD *)&v77 + 1) = v28;
      if ( (unsigned __int8)sub_14087C600(v2, 0, 0, &v77) && (_QWORD)v77 != *((_QWORD *)&v77 + 1) )
      {
        LOBYTE(v58) = 1;
        v59 = sub_140283D60(qword_14E683B38, *(unsigned int *)v77, v58);
        sub_140771610(v3, v59);
      }
      v60 = *(_QWORD *)(v3 + 6960);
      v79 = 0;
      sub_146F1F2F0(v60, 0, &v79);
      sub_146F1FAB0(*(_QWORD *)(v3 + 6960), 0);
    }
    v6 = (char *)v77;
    if ( (_QWORD)v77 )
    {
      v61 = (char *)*((_QWORD *)&v77 + 1);
      if ( (_QWORD)v77 != *((_QWORD *)&v77 + 1) )
      {
        do
        {
          v62 = *((_QWORD *)v6 + 15);
          if ( v62 )
          {
            v63 = 4 * ((*((_QWORD *)v6 + 17) - v62) >> 2);
            if ( v63 >= 0x1000 )
            {
              v63 += 39LL;
              v64 = *(_QWORD *)(v62 - 8);
              v65 = v62 - v64;
              if ( (unsigned __int64)(v65 - 8) > 0x1F )
                sub_148AAF304(v65, v63);
              v62 = v64;
            }
            sub_146E9F3A0(v62, v63);
            *((_QWORD *)v6 + 15) = 0;
            *((_QWORD *)v6 + 16) = 0;
            *((_QWORD *)v6 + 17) = 0;
          }
          v66 = *((_QWORD *)v6 + 11);
          if ( v66 )
          {
            v67 = 4 * ((*((_QWORD *)v6 + 13) - v66) >> 2);
            if ( v67 >= 0x1000 )
            {
              v67 += 39LL;
              v68 = *(_QWORD *)(v66 - 8);
              v69 = v66 - v68;
              if ( (unsigned __int64)(v69 - 8) > 0x1F )
                sub_148AAF304(v69, v67);
              v66 = v68;
            }
            sub_146E9F3A0(v66, v67);
            *((_QWORD *)v6 + 11) = 0;
            *((_QWORD *)v6 + 12) = 0;
            *((_QWORD *)v6 + 13) = 0;
          }
          v70 = *((_QWORD *)v6 + 6);
          if ( v70 )
          {
            v71 = 8 * ((*((_QWORD *)v6 + 8) - v70) >> 3);
            if ( v71 >= 0x1000 )
            {
              v71 += 39LL;
              v72 = *(_QWORD *)(v70 - 8);
              v73 = v70 - v72;
              if ( (unsigned __int64)(v73 - 8) > 0x1F )
                sub_148AAF304(v73, v71);
              v70 = v72;
            }
            sub_146E9F3A0(v70, v71);
            *((_QWORD *)v6 + 6) = 0;
            *((_QWORD *)v6 + 7) = 0;
            *((_QWORD *)v6 + 8) = 0;
          }
          v74 = *((_QWORD *)v6 + 3);
          if ( v74 )
          {
            v21 = (*((_QWORD *)v6 + 5) - v74) & 0xFFFFFFFFFFFFFFF8uLL;
            if ( v21 >= 0x1000 )
            {
              v21 += 39LL;
              v75 = *(_QWORD *)(v74 - 8);
              v23 = v74 - v75;
              if ( (unsigned __int64)(v23 - 8) > 0x1F )
                goto LABEL_107;
              v74 = v75;
            }
            sub_146E9F3A0(v74, v21);
            *((_QWORD *)v6 + 3) = 0;
            *((_QWORD *)v6 + 4) = 0;
            *((_QWORD *)v6 + 5) = 0;
          }
          v6 += 144;
        }
        while ( v6 != v61 );
        v6 = (char *)v77;
      }
      v23 = v78 - (_QWORD)v6;
      v21 = 144 * ((v78 - (__int64)v6) / 144);
      v76 = v6;
      if ( v21 < 0x1000 || (v21 += 39LL, v6 = (char *)*((_QWORD *)v6 - 1), (unsigned __int64)(v76 - v6 - 8) <= 0x1F) )
      {
LABEL_31:
        sub_146E9F3A0(v6, v21);
        v78 = 0;
        v77 = 0;
        return;
      }
LABEL_107:
      sub_148AAF304(v23, v21);
    }
  }
}

