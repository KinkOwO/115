// ins_sub_1407658C0

void __fastcall sub_1407658C0(__int64 a1, __int64 a2)
{
  __int64 v4; // rcx
  __int64 v5; // rdx
  __int128 *v6; // rbx
  __int128 *v7; // rdi
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
  unsigned __int64 v26; // rdi
  unsigned int *v27; // r9
  __int64 v28; // rbx
  __int64 v29; // rax
  int v30; // ecx
  __int64 v31; // rcx
  _QWORD *v32; // rdx
  __int64 v33; // rax
  __int64 v34; // rcx
  __int128 *v35; // rdi
  __int64 v36; // rcx
  unsigned __int64 v37; // rdx
  __int64 v38; // r8
  __int64 v39; // rcx
  __int64 v40; // rcx
  unsigned __int64 v41; // rdx
  __int64 v42; // r8
  __int64 v43; // rcx
  __int64 v44; // rcx
  unsigned __int64 v45; // rdx
  __int64 v46; // r8
  __int64 v47; // rcx
  __int64 v48; // rcx
  __int64 v49; // r8
  char *v50; // rax
  __int128 v51; // [rsp+68h] [rbp-1h] BYREF
  __int64 v52; // [rsp+78h] [rbp+Fh]
  __int128 v53; // [rsp+80h] [rbp+17h] BYREF
  __int64 v54; // [rsp+90h] [rbp+27h]
  __int128 *v55; // [rsp+D0h] [rbp+67h] BYREF
  __int128 *v56; // [rsp+E0h] [rbp+77h]
  __int128 **v57; // [rsp+E8h] [rbp+7Fh]

  v4 = *(_QWORD *)(a1 + 6960);
  if ( v4 && a2 )
  {
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v4 + 16LL))(v4, 0);
    v53 = 0;
    v54 = 0;
    if ( (unsigned __int8)sub_14087C600(a2, 0, 0, &v53) )
    {
      if ( (_QWORD)v53 == *((_QWORD *)&v53 + 1) )
      {
        v6 = (__int128 *)v53;
        if ( !(_QWORD)v53 )
          return;
        v7 = (__int128 *)*((_QWORD *)&v53 + 1);
        v55 = (__int128 *)v53;
        if ( (_QWORD)v53 != *((_QWORD *)&v53 + 1) )
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
                  goto LABEL_71;
                v20 = v22;
              }
              sub_146E9F3A0(v20, v21);
              *((_QWORD *)v6 + 3) = 0;
              *((_QWORD *)v6 + 4) = 0;
              *((_QWORD *)v6 + 5) = 0;
            }
            v6 += 9;
            v55 = v6;
          }
          while ( v6 != v7 );
          v6 = (__int128 *)v53;
        }
        v23 = v54 - (_QWORD)v6;
        v21 = 144 * ((v54 - (__int64)v6) / 144);
        v24 = (char *)v6;
        if ( v21 < 0x1000 )
          goto LABEL_31;
        v21 += 39LL;
        v6 = (__int128 *)*((_QWORD *)v6 - 1);
        if ( (unsigned __int64)(v24 - (char *)v6 - 8) <= 0x1F )
          goto LABEL_31;
        goto LABEL_71;
      }
      LOBYTE(v5) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 6960) + 16LL))(*(_QWORD *)(a1 + 6960), v5);
      sub_146F1E010(*(_QWORD *)(a1 + 6960));
      sub_142268950(*(_QWORD *)(a1 + 6960), 0xFFFFFFFFLL);
      v26 = 0;
      v27 = (unsigned int *)v53;
      if ( (*((_QWORD *)&v53 + 1) - (_QWORD)v53) / 144LL )
      {
        v28 = 0;
        do
        {
          LOBYTE(v25) = 1;
          v29 = sub_140283D60(qword_14E683B38, v27[v28], v25);
          if ( v29 )
          {
            v30 = *(_DWORD *)(v29 + 2012);
            if ( v30 == 22 || v30 == 42 )
            {
              v31 = *(_QWORD *)(a1 + 6960);
              v56 = &v51;
              v51 = 0;
              v52 = 0;
              v57 = &v55;
              v55 = 0;
              v32 = (_QWORD *)(v29 + 496);
              if ( *(_QWORD *)(v29 + 520) >= 8u )
                v32 = (_QWORD *)*v32;
              sub_146F1AD60(
                v31,
                (_DWORD)v32,
                *(_DWORD *)(v28 * 4 + v53),
                0,
                dword_14F1C0918,
                dword_14F1C091C,
                dword_14F1C091C,
                dword_14F1C0920,
                (__int64)&v55,
                1,
                1,
                (__int64)&v51);
            }
          }
          ++v26;
          v28 += 36;
          v27 = (unsigned int *)v53;
        }
        while ( v26 < (*((_QWORD *)&v53 + 1) - (_QWORD)v53) / 144LL );
      }
      LOBYTE(v25) = 1;
      v33 = sub_140283D60(qword_14E683B38, *v27, v25);
      sub_14076FC30(a1, v33);
      v34 = *(_QWORD *)(a1 + 6960);
      v55 = &v51;
      v51 = 0;
      sub_146F1F2F0(v34, 0, &v51);
      sub_146F1FAB0(*(_QWORD *)(a1 + 6960), 0);
    }
    v6 = (__int128 *)v53;
    if ( (_QWORD)v53 )
    {
      v35 = (__int128 *)*((_QWORD *)&v53 + 1);
      v55 = (__int128 *)v53;
      if ( (_QWORD)v53 != *((_QWORD *)&v53 + 1) )
      {
        do
        {
          v36 = *((_QWORD *)v6 + 15);
          if ( v36 )
          {
            v37 = 4 * ((*((_QWORD *)v6 + 17) - v36) >> 2);
            if ( v37 >= 0x1000 )
            {
              v37 += 39LL;
              v38 = *(_QWORD *)(v36 - 8);
              v39 = v36 - v38;
              if ( (unsigned __int64)(v39 - 8) > 0x1F )
                sub_148AAF304(v39, v37);
              v36 = v38;
            }
            sub_146E9F3A0(v36, v37);
            *((_QWORD *)v6 + 15) = 0;
            *((_QWORD *)v6 + 16) = 0;
            *((_QWORD *)v6 + 17) = 0;
          }
          v40 = *((_QWORD *)v6 + 11);
          if ( v40 )
          {
            v41 = 4 * ((*((_QWORD *)v6 + 13) - v40) >> 2);
            if ( v41 >= 0x1000 )
            {
              v41 += 39LL;
              v42 = *(_QWORD *)(v40 - 8);
              v43 = v40 - v42;
              if ( (unsigned __int64)(v43 - 8) > 0x1F )
                sub_148AAF304(v43, v41);
              v40 = v42;
            }
            sub_146E9F3A0(v40, v41);
            *((_QWORD *)v6 + 11) = 0;
            *((_QWORD *)v6 + 12) = 0;
            *((_QWORD *)v6 + 13) = 0;
          }
          v44 = *((_QWORD *)v6 + 6);
          if ( v44 )
          {
            v45 = 8 * ((*((_QWORD *)v6 + 8) - v44) >> 3);
            if ( v45 >= 0x1000 )
            {
              v45 += 39LL;
              v46 = *(_QWORD *)(v44 - 8);
              v47 = v44 - v46;
              if ( (unsigned __int64)(v47 - 8) > 0x1F )
                sub_148AAF304(v47, v45);
              v44 = v46;
            }
            sub_146E9F3A0(v44, v45);
            *((_QWORD *)v6 + 6) = 0;
            *((_QWORD *)v6 + 7) = 0;
            *((_QWORD *)v6 + 8) = 0;
          }
          v48 = *((_QWORD *)v6 + 3);
          if ( v48 )
          {
            v21 = (*((_QWORD *)v6 + 5) - v48) & 0xFFFFFFFFFFFFFFF8uLL;
            if ( v21 >= 0x1000 )
            {
              v21 += 39LL;
              v49 = *(_QWORD *)(v48 - 8);
              v23 = v48 - v49;
              if ( (unsigned __int64)(v23 - 8) > 0x1F )
                goto LABEL_71;
              v48 = v49;
            }
            sub_146E9F3A0(v48, v21);
            *((_QWORD *)v6 + 3) = 0;
            *((_QWORD *)v6 + 4) = 0;
            *((_QWORD *)v6 + 5) = 0;
          }
          v6 += 9;
          v55 = v6;
        }
        while ( v6 != v35 );
        v6 = (__int128 *)v53;
      }
      v23 = v54 - (_QWORD)v6;
      v21 = 144 * ((v54 - (__int64)v6) / 144);
      v50 = (char *)v6;
      if ( v21 < 0x1000
        || (v21 += 39LL, v6 = (__int128 *)*((_QWORD *)v6 - 1), (unsigned __int64)(v50 - (char *)v6 - 8) <= 0x1F) )
      {
LABEL_31:
        sub_146E9F3A0(v6, v21);
        v54 = 0;
        v53 = 0;
        return;
      }
LABEL_71:
      sub_148AAF304(v23, v21);
    }
  }
}

