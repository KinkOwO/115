// ins_sub_14075B9A0

__int64 __fastcall sub_14075B9A0(__int64 a1, __int64 a2)
{
  __int64 v4; // rsi
  __int64 v5; // r8
  int v6; // eax
  unsigned int v7; // esi
  char *v8; // rbx
  char *v9; // rdi
  __int64 v10; // rcx
  unsigned __int64 v11; // rdx
  __int64 v12; // r8
  __int64 v13; // rcx
  __int64 v14; // rcx
  unsigned __int64 v15; // rdx
  __int64 v16; // r8
  __int64 v17; // rcx
  __int64 v18; // rcx
  unsigned __int64 v19; // rdx
  __int64 v20; // r8
  __int64 v21; // rcx
  __int64 v22; // rcx
  unsigned __int64 v23; // rdx
  __int64 v24; // r8
  __int64 v25; // rcx
  char *v26; // rax
  __int128 v28; // [rsp+28h] [rbp-20h] BYREF
  __int64 v29; // [rsp+38h] [rbp-10h]

  v4 = a2;
  v28 = 0;
  v29 = 0;
  if ( (unsigned __int8)sub_14087C600(a2, 0, 0, &v28, -2) )
  {
    LOBYTE(v5) = 1;
    v4 = sub_140283D60(qword_14E683B38, *(unsigned int *)v28, v5);
  }
  v6 = *(_DWORD *)(v4 + 2012);
  if ( v6 == 22 || v6 == 42 )
  {
    sub_14076B140(a1);
    sub_1407658C0(a1, a2);
    sub_14075E6F0(a1, a2);
    v7 = 29;
  }
  else
  {
    v7 = 0;
  }
  v8 = (char *)v28;
  if ( (_QWORD)v28 )
  {
    v9 = (char *)*((_QWORD *)&v28 + 1);
    if ( (_QWORD)v28 != *((_QWORD *)&v28 + 1) )
    {
      do
      {
        v10 = *((_QWORD *)v8 + 15);
        if ( v10 )
        {
          v11 = 4 * ((*((_QWORD *)v8 + 17) - v10) >> 2);
          if ( v11 >= 0x1000 )
          {
            v11 += 39LL;
            v12 = *(_QWORD *)(v10 - 8);
            v13 = v10 - v12;
            if ( (unsigned __int64)(v13 - 8) > 0x1F )
              sub_148AAF304(v13, v11);
            v10 = v12;
          }
          sub_146E9F3A0(v10, v11);
          *((_QWORD *)v8 + 15) = 0;
          *((_QWORD *)v8 + 16) = 0;
          *((_QWORD *)v8 + 17) = 0;
        }
        v14 = *((_QWORD *)v8 + 11);
        if ( v14 )
        {
          v15 = 4 * ((*((_QWORD *)v8 + 13) - v14) >> 2);
          if ( v15 >= 0x1000 )
          {
            v15 += 39LL;
            v16 = *(_QWORD *)(v14 - 8);
            v17 = v14 - v16;
            if ( (unsigned __int64)(v17 - 8) > 0x1F )
              sub_148AAF304(v17, v15);
            v14 = v16;
          }
          sub_146E9F3A0(v14, v15);
          *((_QWORD *)v8 + 11) = 0;
          *((_QWORD *)v8 + 12) = 0;
          *((_QWORD *)v8 + 13) = 0;
        }
        v18 = *((_QWORD *)v8 + 6);
        if ( v18 )
        {
          v19 = 8 * ((*((_QWORD *)v8 + 8) - v18) >> 3);
          if ( v19 >= 0x1000 )
          {
            v19 += 39LL;
            v20 = *(_QWORD *)(v18 - 8);
            v21 = v18 - v20;
            if ( (unsigned __int64)(v21 - 8) > 0x1F )
              sub_148AAF304(v21, v19);
            v18 = v20;
          }
          sub_146E9F3A0(v18, v19);
          *((_QWORD *)v8 + 6) = 0;
          *((_QWORD *)v8 + 7) = 0;
          *((_QWORD *)v8 + 8) = 0;
        }
        v22 = *((_QWORD *)v8 + 3);
        if ( v22 )
        {
          v23 = (*((_QWORD *)v8 + 5) - v22) & 0xFFFFFFFFFFFFFFF8uLL;
          if ( v23 >= 0x1000 )
          {
            v23 += 39LL;
            v24 = *(_QWORD *)(v22 - 8);
            v25 = v22 - v24;
            if ( (unsigned __int64)(v25 - 8) > 0x1F )
              goto LABEL_36;
            v22 = v24;
          }
          sub_146E9F3A0(v22, v23);
          *((_QWORD *)v8 + 3) = 0;
          *((_QWORD *)v8 + 4) = 0;
          *((_QWORD *)v8 + 5) = 0;
        }
        v8 += 144;
      }
      while ( v8 != v9 );
      v8 = (char *)v28;
    }
    v25 = v29 - (_QWORD)v8;
    v23 = 144 * ((v29 - (__int64)v8) / 144);
    v26 = v8;
    if ( v23 >= 0x1000 )
    {
      v23 += 39LL;
      v8 = (char *)*((_QWORD *)v8 - 1);
      if ( (unsigned __int64)(v26 - v8 - 8) > 0x1F )
LABEL_36:
        sub_148AAF304(v25, v23);
    }
    sub_146E9F3A0(v8, v23);
    v28 = 0;
    v29 = 0;
  }
  return v7;
}

