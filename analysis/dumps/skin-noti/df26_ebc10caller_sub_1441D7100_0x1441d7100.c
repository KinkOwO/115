// ebc10caller_sub_1441D7100_0x1441d7100

__int64 __fastcall sub_1441D7100(__int64 *a1, _QWORD *a2, int a3)
{
  _QWORD *v3; // r13
  __int64 v5; // rax
  __int64 v6; // rax
  __int64 v7; // r12
  unsigned int *v8; // r15
  char *v9; // rdi
  __int64 v10; // rcx
  __int64 v11; // rax
  void (__fastcall ***v12)(_QWORD); // rcx
  char **v13; // rax
  __int64 v14; // rdx
  char *v15; // rbx
  char *v16; // r14
  __int64 v17; // rcx
  unsigned __int64 v18; // rdx
  __int64 v19; // rcx
  __int64 v20; // rcx
  unsigned __int64 v21; // rdx
  char *v22; // rax
  __int64 v23; // r12
  __int64 v24; // rax
  void (__fastcall ***v25)(_QWORD); // rcx
  __int64 v26; // r14
  __int64 v27; // r15
  __int64 v28; // rbx
  __int64 v29; // rdi
  __int64 v30; // rbx
  int v31; // eax
  __int64 v32; // rax
  __int128 v34; // [rsp+58h] [rbp-9h] BYREF
  char *v35; // [rsp+68h] [rbp+7h]
  _OWORD v36[2]; // [rsp+70h] [rbp+Fh] BYREF
  int v37; // [rsp+D0h] [rbp+6Fh] BYREF

  v3 = a2;
  if ( a3 == 13 )
  {
    v5 = a1[9];
    if ( v5 && v5 == *a2 )
    {
      (*(void (__fastcall **)(__int64 *))(*a1 + 112))(a1);
    }
    else
    {
      v6 = a1[11];
      if ( v6 && v6 == *a2 )
        (*(void (__fastcall **)(__int64 *))(*a1 + 104))(a1);
    }
    v7 = 0;
    v8 = (unsigned int *)(a1 + 26);
    do
    {
      if ( *v3 == *((_QWORD *)v8 + 1) )
      {
        v34 = 0;
        v9 = 0;
        v35 = 0;
        v10 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          v11 = sub_146E8BA20(1472);
          if ( v11 )
            v12 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v11);
          else
            v12 = 0;
          qword_14E638F28 = (__int64)v12;
          (**v12)(v12);
          v10 = qword_14E638F28;
        }
        v13 = (char **)sub_1444EBC10(v10, v36, 7);
        if ( &v34 == (__int128 *)v13 )
        {
          v16 = (char *)*((_QWORD *)&v34 + 1);
          v15 = (char *)v34;
        }
        else
        {
          v15 = *v13;
          *(_QWORD *)&v34 = *v13;
          v16 = v13[1];
          *((_QWORD *)&v34 + 1) = v16;
          v9 = v13[2];
          v35 = v9;
          *v13 = 0;
          v13[1] = 0;
          v13[2] = 0;
        }
        v17 = *(_QWORD *)&v36[0];
        if ( *(_QWORD *)&v36[0] )
        {
          v18 = 4 * ((__int64)(*(_QWORD *)&v36[1] - *(_QWORD *)&v36[0]) >> 2);
          if ( v18 >= 0x1000 )
          {
            v18 += 39LL;
            v17 = *(_QWORD *)(*(_QWORD *)&v36[0] - 8LL);
            if ( (unsigned __int64)(*(_QWORD *)&v36[0] - v17 - 8) > 0x1F )
              sub_148AAF304(v17, v18);
          }
          sub_146E9F3A0(v17, v18);
          memset(v36, 0, 24);
        }
        LOBYTE(v14) = v15 == v16 || *(_DWORD *)v15 != *v8;
        v19 = a1[19];
        if ( v19 )
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v19 + 24LL))(v19, v14);
        sub_1441E0820(a1[1], 7, *v8, 0);
        (*(void (__fastcall **)(__int64 *, _QWORD))(*a1 + 32))(a1, 0);
        if ( v15 )
        {
          v21 = 4 * ((v9 - v15) >> 2);
          v22 = v15;
          if ( v21 >= 0x1000 )
          {
            v21 += 39LL;
            v15 = (char *)*((_QWORD *)v15 - 1);
            if ( (unsigned __int64)(v22 - v15 - 8) > 0x1F )
              goto LABEL_54;
          }
          sub_146E9F3A0(v15, v21);
          v34 = 0;
          v35 = 0;
        }
      }
      ++v7;
      v8 += 44;
    }
    while ( v7 < 9 );
    if ( a1[19] == *v3 )
    {
      memset(v36, 0, 24);
      v37 = **(_DWORD **)(a1[1] + 3520);
      sub_140154010(v36, 0, &v37);
      v23 = qword_14E638F28;
      if ( !qword_14E638F28 )
      {
        v24 = sub_146E8BA20(1472);
        if ( v24 )
          v25 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v24);
        else
          v25 = 0;
        qword_14E638F28 = (__int64)v25;
        (**v25)(v25);
        v23 = qword_14E638F28;
      }
      v34 = 0;
      v35 = 0;
      v26 = *(_QWORD *)&v36[0];
      if ( *(_QWORD *)&v36[0] != *((_QWORD *)&v36[0] + 1) )
      {
        v27 = *((_QWORD *)&v36[0] + 1) - *(_QWORD *)&v36[0];
        v28 = (__int64)(*((_QWORD *)&v36[0] + 1) - *(_QWORD *)&v36[0]) >> 2;
        *(_QWORD *)&v34 = sub_140157580(&v34, v28);
        *((_QWORD *)&v34 + 1) = v34;
        v29 = 4 * v28;
        v35 = (char *)(4 * v28 + v34);
        v30 = v34;
        sub_148AA1E60(v34, v26, v27);
        *((_QWORD *)&v34 + 1) = v30 + v29;
      }
      sub_1444F1090(v23, 7, &v34);
      v31 = sub_146E8C7D0(&unk_149242AE8);
      sub_145A31380(v31, -1, 0, 0, -1, -1, 0);
      v20 = a1[19];
      if ( v20 )
        (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v20 + 24LL))(v20, 0);
      if ( v26 )
      {
        v21 = (*(_QWORD *)&v36[1] - v26) & 0xFFFFFFFFFFFFFFFCuLL;
        v32 = v26;
        if ( v21 >= 0x1000 )
        {
          v21 += 39LL;
          v26 = *(_QWORD *)(v26 - 8);
          if ( (unsigned __int64)(v32 - v26 - 8) > 0x1F )
LABEL_54:
            sub_148AAF304(v20, v21);
        }
        sub_146E9F3A0(v26, v21);
        memset(v36, 0, 24);
      }
    }
  }
  else if ( a3 == 24 )
  {
    if ( *a2 == a1[14] )
    {
      a2 = 0;
    }
    else
    {
      if ( *a2 != a1[5] )
        return 0;
      LOBYTE(a2) = 1;
    }
    (*(void (__fastcall **)(__int64 *, _QWORD *))(*a1 + 32))(a1, a2);
  }
  return 0;
}

