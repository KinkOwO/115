// ins_sub_14075E6F0

void __fastcall sub_14075E6F0(__int64 a1, __int64 a2)
{
  __int64 v2; // rdi
  __int64 v4; // r8
  int v5; // eax
  unsigned int v6; // eax
  __int64 v7; // rax
  __int64 v8; // rdx
  __int64 v9; // rdi
  int v10; // eax
  __int64 v11; // rdx
  unsigned __int64 v12; // rcx
  __int64 v13; // rax
  bool v14; // di
  __int64 v15; // rcx
  __int64 v16; // rcx
  char *v17; // rbx
  char *v18; // rdi
  __int64 v19; // rcx
  unsigned __int64 v20; // rdx
  __int64 v21; // r8
  __int64 v22; // rcx
  __int64 v23; // rcx
  unsigned __int64 v24; // rdx
  __int64 v25; // r8
  __int64 v26; // rcx
  __int64 v27; // rcx
  unsigned __int64 v28; // rdx
  __int64 v29; // r8
  __int64 v30; // rcx
  __int64 v31; // rcx
  unsigned __int64 v32; // rdx
  __int64 v33; // r8
  __int64 v34; // rcx
  char *v35; // rax
  __int128 v36; // [rsp+28h] [rbp-20h] BYREF
  __int64 v37; // [rsp+38h] [rbp-10h]

  if ( a2 )
  {
    v2 = a2;
    if ( *(_QWORD *)(a1 + 6960) )
    {
      v36 = 0;
      v37 = 0;
      if ( (unsigned __int8)sub_14087C600(a2, 0, 0, &v36, -2) )
      {
        LOBYTE(v4) = 1;
        v2 = sub_140283D60(qword_14E683B38, *(unsigned int *)v36, v4);
      }
      v5 = *(_DWORD *)(v2 + 2012);
      if ( v5 == 22 || v5 == 42 )
      {
        if ( (int)sub_146F1E6E0(*(_QWORD *)(a1 + 6960)) > 0 )
        {
          v6 = sub_14206BB60(*(_QWORD *)(a1 + 6960));
          v7 = sub_145ABB480(v6);
          v9 = v7;
          if ( v7 )
          {
            v10 = *(_DWORD *)(v7 + 2012);
            if ( v10 == 22 || v10 == 42 )
            {
              LOBYTE(v8) = 1;
              sub_14076D040(a1, v8);
              sub_14076C9B0(a1, v9 + 456, v9 + 432);
              if ( *(_BYTE *)(a1 + 7008) )
                sub_14076FC30(a1, v9);
              if ( (int)sub_146F1E6E0(*(_QWORD *)(a1 + 6976)) > 0 )
                *(_DWORD *)(a1 + 7952) = sub_14206BB60(*(_QWORD *)(a1 + 6976));
            }
          }
        }
        v11 = *(_QWORD *)(a1 + 7928);
        v12 = *(int *)(a1 + 7952);
        if ( v12 < (*(_QWORD *)(a1 + 7936) - v11) >> 5 )
          v13 = (__int64)(*(_QWORD *)(32 * v12 + v11 + 8) - *(_QWORD *)(32 * v12 + v11)) >> 4;
        else
          LODWORD(v13) = 0;
        v14 = (int)v13 > 1;
        v15 = *(_QWORD *)(a1 + 6904);
        if ( v15 )
          (*(void (__fastcall **)(__int64, bool))(*(_QWORD *)v15 + 16LL))(v15, v14);
        v16 = *(_QWORD *)(a1 + 6920);
        if ( v16 )
          (*(void (__fastcall **)(__int64, bool))(*(_QWORD *)v16 + 16LL))(v16, v14);
      }
      v17 = (char *)v36;
      if ( (_QWORD)v36 )
      {
        v18 = (char *)*((_QWORD *)&v36 + 1);
        if ( (_QWORD)v36 != *((_QWORD *)&v36 + 1) )
        {
          do
          {
            v19 = *((_QWORD *)v17 + 15);
            if ( v19 )
            {
              v20 = 4 * ((*((_QWORD *)v17 + 17) - v19) >> 2);
              if ( v20 >= 0x1000 )
              {
                v20 += 39LL;
                v21 = *(_QWORD *)(v19 - 8);
                v22 = v19 - v21;
                if ( (unsigned __int64)(v22 - 8) > 0x1F )
                  sub_148AAF304(v22, v20);
                v19 = v21;
              }
              sub_146E9F3A0(v19, v20);
              *((_QWORD *)v17 + 15) = 0;
              *((_QWORD *)v17 + 16) = 0;
              *((_QWORD *)v17 + 17) = 0;
            }
            v23 = *((_QWORD *)v17 + 11);
            if ( v23 )
            {
              v24 = 4 * ((*((_QWORD *)v17 + 13) - v23) >> 2);
              if ( v24 >= 0x1000 )
              {
                v24 += 39LL;
                v25 = *(_QWORD *)(v23 - 8);
                v26 = v23 - v25;
                if ( (unsigned __int64)(v26 - 8) > 0x1F )
                  sub_148AAF304(v26, v24);
                v23 = v25;
              }
              sub_146E9F3A0(v23, v24);
              *((_QWORD *)v17 + 11) = 0;
              *((_QWORD *)v17 + 12) = 0;
              *((_QWORD *)v17 + 13) = 0;
            }
            v27 = *((_QWORD *)v17 + 6);
            if ( v27 )
            {
              v28 = 8 * ((*((_QWORD *)v17 + 8) - v27) >> 3);
              if ( v28 >= 0x1000 )
              {
                v28 += 39LL;
                v29 = *(_QWORD *)(v27 - 8);
                v30 = v27 - v29;
                if ( (unsigned __int64)(v30 - 8) > 0x1F )
                  sub_148AAF304(v30, v28);
                v27 = v29;
              }
              sub_146E9F3A0(v27, v28);
              *((_QWORD *)v17 + 6) = 0;
              *((_QWORD *)v17 + 7) = 0;
              *((_QWORD *)v17 + 8) = 0;
            }
            v31 = *((_QWORD *)v17 + 3);
            if ( v31 )
            {
              v32 = (*((_QWORD *)v17 + 5) - v31) & 0xFFFFFFFFFFFFFFF8uLL;
              if ( v32 >= 0x1000 )
              {
                v32 += 39LL;
                v33 = *(_QWORD *)(v31 - 8);
                v34 = v31 - v33;
                if ( (unsigned __int64)(v34 - 8) > 0x1F )
                  goto LABEL_51;
                v31 = v33;
              }
              sub_146E9F3A0(v31, v32);
              *((_QWORD *)v17 + 3) = 0;
              *((_QWORD *)v17 + 4) = 0;
              *((_QWORD *)v17 + 5) = 0;
            }
            v17 += 144;
          }
          while ( v17 != v18 );
          v17 = (char *)v36;
        }
        v34 = v37 - (_QWORD)v17;
        v32 = 144 * ((v37 - (__int64)v17) / 144);
        v35 = v17;
        if ( v32 >= 0x1000 )
        {
          v32 += 39LL;
          v17 = (char *)*((_QWORD *)v17 - 1);
          if ( (unsigned __int64)(v35 - v17 - 8) > 0x1F )
LABEL_51:
            sub_148AAF304(v34, v32);
        }
        sub_146E9F3A0(v17, v32);
        v36 = 0;
        v37 = 0;
      }
    }
  }
}

