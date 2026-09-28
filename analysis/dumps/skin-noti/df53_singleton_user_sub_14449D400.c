// singleton_user_sub_14449D400

void sub_14449D400()
{
  unsigned __int8 i; // r12
  char *v1; // rsi
  int v2; // edi
  char *v3; // rbx
  char *v4; // r14
  __int64 v5; // r15
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx
  int v8; // edi
  int v9; // ebx
  int v10; // r9d
  int v11; // edx
  __int64 v12; // r10
  __int64 v13; // r11
  _DWORD *v14; // r8
  __int64 v15; // r10
  __int64 v16; // rax
  void (__fastcall ***v17)(_QWORD); // rcx
  __int64 v18; // r8
  __int64 v19; // r9
  __int64 v20; // rdx
  __int64 v21; // rdx
  __int64 v22; // rax
  void (__fastcall ***v23)(_QWORD); // rcx
  __int64 v24; // rcx
  unsigned __int64 v25; // rdx
  char *v26; // rax
  __int64 v27; // rcx
  __int64 v28; // rax
  void (__fastcall ***v29)(_QWORD); // rcx
  __int64 v30; // rax
  void (__fastcall ***v31)(_QWORD); // rcx
  __int64 v32; // rax
  __int64 v33; // rbx
  __int64 v34; // rax
  unsigned __int64 v35; // rdx
  __int64 v36; // rcx
  unsigned __int8 v37; // [rsp+20h] [rbp-60h] BYREF
  unsigned __int8 v38; // [rsp+21h] [rbp-5Fh] BYREF
  unsigned __int8 v39; // [rsp+22h] [rbp-5Eh] BYREF
  unsigned __int8 v40; // [rsp+23h] [rbp-5Dh] BYREF
  char v41[4]; // [rsp+24h] [rbp-5Ch] BYREF
  int v42; // [rsp+28h] [rbp-58h] BYREF
  int v43; // [rsp+2Ch] [rbp-54h] BYREF
  __int64 v44; // [rsp+30h] [rbp-50h] BYREF
  __int64 v45; // [rsp+38h] [rbp-48h]
  __int64 v46; // [rsp+40h] [rbp-40h]
  __int64 v47; // [rsp+48h] [rbp-38h]
  __int64 v48; // [rsp+50h] [rbp-30h]
  __int128 v49; // [rsp+58h] [rbp-28h] BYREF
  char *v50; // [rsp+68h] [rbp-18h]
  unsigned __int64 v51; // [rsp+70h] [rbp-10h]

  v46 = -2;
  v40 = 0;
  v37 = 0;
  v39 = 0;
  v38 = 0;
  v42 = 0;
  v41[0] = 0;
  v43 = 0;
  sub_146EA09F0(&v40);
  for ( i = 0; i < v40; ++i )
  {
    sub_146EA09F0(&v37);
    sub_146EA09F0(&v39);
    sub_146EA09F0(&v38);
    v49 = 0;
    v1 = 0;
    v50 = 0;
    v2 = 0;
    v3 = 0;
    if ( v38 )
    {
      do
      {
        sub_146EA0BA0(&v42);
        sub_146EA09F0(v41);
        LODWORD(v44) = v42;
        HIDWORD(v44) = (unsigned __int8)v41[0];
        if ( v3 == v1 )
        {
          sub_140183FB0(&v49, v3, &v44);
          v1 = v50;
          v3 = (char *)*((_QWORD *)&v49 + 1);
        }
        else
        {
          *(_QWORD *)v3 = v44;
          v3 += 8;
          *((_QWORD *)&v49 + 1) = v3;
        }
        ++v2;
      }
      while ( v2 < v38 );
    }
    v4 = (char *)v49;
    if ( (unsigned __int64)((__int64)&v3[-v49] >> 3) < 2 )
    {
      v15 = qword_14E659EA8;
      if ( !qword_14E659EA8 )
      {
        v16 = sub_146E8BA20(496);
        v48 = v16;
        if ( v16 )
          v17 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v16);
        else
          v17 = 0;
        qword_14E659EA8 = (__int64)v17;
        (**v17)(v17);
        v15 = qword_14E659EA8;
      }
      v18 = v39;
      v19 = v37;
      v20 = v15 + 24LL * v37;
      *(_DWORD *)(v20 + 132) = v37;
      *(_DWORD *)(v20 + 128) = v18;
      *(_QWORD *)(v20 + 136) = -1;
      *(_DWORD *)(v15 + 24 * v19 + 144) = -1;
      *(_DWORD *)(v20 + 148) = -1;
      *(_DWORD *)(v15 + 4 * v18 + 316) = v19;
    }
    else
    {
      v5 = qword_14E659EA8;
      if ( !qword_14E659EA8 )
      {
        v6 = sub_146E8BA20(496);
        v47 = v6;
        if ( v6 )
          v7 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v6);
        else
          v7 = 0;
        qword_14E659EA8 = (__int64)v7;
        (**v7)(v7);
        v5 = qword_14E659EA8;
      }
      v8 = *((_DWORD *)v4 + 3);
      v9 = *((_DWORD *)v4 + 2);
      v10 = *((_DWORD *)v4 + 1);
      v11 = *(_DWORD *)v4;
      v12 = v39;
      v13 = v37;
      v14 = (_DWORD *)(v5 + 24LL * v37);
      v14[33] = v37;
      v14[32] = v12;
      v14[34] = v11;
      v14[35] = v10;
      *(_DWORD *)(v5 + 24 * v13 + 144) = v9;
      v14[37] = v8;
      *(_DWORD *)(v5 + 4 * v12 + 316) = v13;
    }
    sub_146EA0BA0(&v43);
    v21 = qword_14E659EA8;
    if ( !qword_14E659EA8 )
    {
      v22 = sub_146E8BA20(496);
      v45 = v22;
      if ( v22 )
        v23 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v22);
      else
        v23 = 0;
      qword_14E659EA8 = (__int64)v23;
      (**v23)(v23);
      v21 = qword_14E659EA8;
    }
    v24 = v37;
    *(_DWORD *)(v21 + 8LL * v37 + 68) = v43;
    if ( v4 )
    {
      v25 = 8 * ((v1 - v4) >> 3);
      v26 = v4;
      if ( v25 >= 0x1000 )
      {
        v25 += 39LL;
        v4 = (char *)*((_QWORD *)v4 - 1);
        if ( (unsigned __int64)(v26 - v4 - 8) > 0x1F )
          sub_148AAF304(v24, v25);
      }
      sub_146E9F3A0(v4, v25);
      v49 = 0;
      v50 = 0;
    }
  }
  v27 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v28 = sub_146E8BA20(496);
    v45 = v28;
    if ( v28 )
      v29 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v28);
    else
      v29 = 0;
    qword_14E659EA8 = (__int64)v29;
    (**v29)(v29);
    v27 = qword_14E659EA8;
  }
  *(_DWORD *)(v27 + 416) = 1;
  sub_146E9FBD0(v27 + 424, 0, 0);
  if ( !qword_14E659EA8 )
  {
    v30 = sub_146E8BA20(496);
    v45 = v30;
    if ( v30 )
      v31 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v30);
    else
      v31 = 0;
    qword_14E659EA8 = (__int64)v31;
    (**v31)(v31);
  }
  v32 = sub_1459A9080(qword_14E66C090);
  if ( v32 )
  {
    v33 = sub_145B2DEF0(v32);
    if ( v33 )
    {
      v34 = sub_146E8C7D0(&unk_14A300D20);
      *(_QWORD *)&v49 = 0;
      v50 = 0;
      v51 = 7;
      sub_14014C8B0(&v49, v34);
      sub_144CCDF70(v33, &v49, 1);
      if ( v51 >= 8 )
      {
        v35 = 2 * v51 + 2;
        v36 = v49;
        if ( v35 >= 0x1000 )
        {
          v35 = 2 * v51 + 41;
          v36 = *(_QWORD *)(v49 - 8);
          if ( (unsigned __int64)(v49 - v36 - 8) > 0x1F )
            sub_148AAF304(v36, v35);
        }
        sub_146E9F3A0(v36, v35);
      }
      v50 = 0;
      v51 = 7;
      LOWORD(v49) = 0;
    }
  }
}

