__int64 sub_144360950()
{
  __int64 v0; // r15
  __int64 v1; // rcx
  __int64 v2; // rax
  void (__fastcall ***v3)(_QWORD); // rcx
  __int64 *v4; // rdx
  int *v5; // rax
  __int64 *v6; // rcx
  __int64 v7; // r14
  __int64 v8; // rdi
  __int64 v9; // rbx
  int v10; // esi
  __int64 v11; // r9
  __int64 *v12; // r8
  char *v13; // rax
  __int64 *v14; // rcx
  int v15; // edi
  __int64 v16; // r9
  __int64 *v17; // r8
  char *v18; // rax
  __int64 *v19; // rcx
  __int64 v20; // rcx
  __int64 v21; // rax
  void (__fastcall ***v22)(_QWORD); // rcx
  __int64 v23; // rbx
  __int64 v24; // rax
  void (__fastcall ***v25)(_QWORD); // rcx
  __int64 v26; // rax
  int v27; // eax
  __int64 v28; // rax
  __int64 v29; // rax
  void (__fastcall ***v30)(_QWORD); // rcx
  __int64 result; // rax
  __int64 v32; // rcx
  __int64 v33; // rax
  void (__fastcall ***v34)(_QWORD); // rcx
  __int64 v35; // r8
  unsigned __int64 v36; // rdx
  __int64 v37; // r8
  unsigned __int64 v38; // rdx
  unsigned __int8 v39; // [rsp+30h] [rbp-29h] BYREF
  char v40; // [rsp+31h] [rbp-28h] BYREF
  _BYTE v41[6]; // [rsp+32h] [rbp-27h] BYREF
  _BYTE v42[24]; // [rsp+38h] [rbp-21h] BYREF
  __int128 v43; // [rsp+50h] [rbp-9h] BYREF
  __int64 v44; // [rsp+60h] [rbp+7h]
  __int128 v45; // [rsp+68h] [rbp+Fh] BYREF
  __int64 v46; // [rsp+78h] [rbp+1Fh]
  __int64 v47; // [rsp+80h] [rbp+27h]
  __int64 v48; // [rsp+D0h] [rbp+77h] BYREF
  signed __int8 v49; // [rsp+D8h] [rbp+7Fh] BYREF

  v47 = -2;
  v0 = sub_144CEFF10(qword_14E683C30);
  v1 = qword_14E650FE8;
  if ( !qword_14E650FE8 )
  {
    v2 = sub_146E8BA20(96);
    v48 = v2;
    if ( v2 )
      v3 = (void (__fastcall ***)(_QWORD))sub_14435F9D0(v2);
    else
      v3 = 0;
    qword_14E650FE8 = (__int64)v3;
    (**v3)(v3);
    v1 = qword_14E650FE8;
  }
  v4 = *(__int64 **)(v1 + 64);
  v5 = (int *)v4[1];
  v6 = v4;
  while ( !*((_BYTE *)v5 + 25) )
  {
    if ( v5[8] >= 1 )
    {
      v6 = (__int64 *)v5;
      v5 = *(int **)v5;
    }
    else
    {
      v5 = (int *)*((_QWORD *)v5 + 2);
    }
  }
  if ( !*((_BYTE *)v6 + 25) && *((int *)v6 + 8) <= 1 && v6 != v4 )
  {
    v6[6] = v6[5];
    v6[9] = v6[8];
  }
  v43 = 0u;
  v7 = 0;
  v44 = 0;
  v45 = 0u;
  v46 = 0;
  v8 = 0;
  v9 = 0;
  sub_146EA09F0(&v48, 1);
  v10 = 0;
  if ( (_BYTE)v48 )
  {
    do
    {
      sub_146EA09F0(&v49, 1);
      sub_146EA09F0(&v39, 1);
      sub_146EA09F0(&v40, 1);
      sub_146EA09F0(v41, 1);
      if ( v0 )
      {
        v12 = *(__int64 **)(v0 + 1792);
        v13 = (char *)v12[1];
        v14 = v12;
        while ( !v13[25] )
        {
          if ( v13[28] >= v49 )
          {
            v14 = (__int64 *)v13;
            v13 = *(char **)v13;
          }
          else
          {
            v13 = (char *)*((_QWORD *)v13 + 2);
          }
        }
        if ( *((_BYTE *)v14 + 25) || v49 < *((char *)v14 + 28) )
          v14 = *(__int64 **)(v0 + 1792);
        if ( v14 != v12 )
        {
          *(_DWORD *)v42 = (unsigned __int8)v49;
          *(_DWORD *)&v42[4] = v39;
          v42[20] = v40 != 0;
          v42[21] = v41[0] != 0;
          *(_QWORD *)&v42[12] = v14[4];
          *(_DWORD *)&v42[8] = *(_DWORD *)&v42[12];
          if ( v8 == v7 )
          {
            sub_140C39140(&v43, v8, v42, v11);
            v7 = v44;
            v8 = *((_QWORD *)&v43 + 1);
          }
          else
          {
            *(_OWORD *)v8 = *(_OWORD *)v42;
            *(_QWORD *)(v8 + 16) = *(_QWORD *)&v42[16];
            v8 += 24;
            *((_QWORD *)&v43 + 1) = v8;
          }
        }
      }
      ++v10;
    }
    while ( v10 < (unsigned __int8)v48 );
    v9 = *((_QWORD *)&v45 + 1);
  }
  sub_146EA09F0(&v48, 1);
  v15 = 0;
  if ( (_BYTE)v48 )
  {
    do
    {
      sub_146EA09F0(&v49, 1);
      sub_146EA09F0(v41, 1);
      sub_146EA09F0(&v40, 1);
      sub_146EA09F0(&v39, 1);
      if ( v0 )
      {
        v17 = *(__int64 **)(v0 + 1808);
        v18 = (char *)v17[1];
        v19 = v17;
        while ( !v18[25] )
        {
          if ( v18[28] >= v49 )
          {
            v19 = (__int64 *)v18;
            v18 = *(char **)v18;
          }
          else
          {
            v18 = (char *)*((_QWORD *)v18 + 2);
          }
        }
        if ( *((_BYTE *)v19 + 25) || v49 < *((char *)v19 + 28) )
          v19 = *(__int64 **)(v0 + 1808);
        if ( v19 != v17 )
        {
          *(_DWORD *)v42 = (unsigned __int8)v49;
          *(_DWORD *)&v42[4] = v41[0];
          v42[20] = v40 != 0;
          v42[21] = v39 != 0;
          *(_QWORD *)&v42[12] = v19[4];
          *(_DWORD *)&v42[8] = *(_DWORD *)&v42[12];
          if ( v9 == v46 )
          {
            sub_140C39140(&v45, v9, v42, v16);
            v9 = *((_QWORD *)&v45 + 1);
          }
          else
          {
            *(_OWORD *)v9 = *(_OWORD *)v42;
            *(_QWORD *)(v9 + 16) = *(_QWORD *)&v42[16];
            v9 += 24;
            *((_QWORD *)&v45 + 1) = v9;
          }
        }
      }
      ++v15;
    }
    while ( v15 < (unsigned __int8)v48 );
  }
  v20 = qword_14E650FE8;
  if ( !qword_14E650FE8 )
  {
    v21 = sub_146E8BA20(96);
    v48 = v21;
    if ( v21 )
      v22 = (void (__fastcall ***)(_QWORD))sub_14435F9D0(v21);
    else
      v22 = 0;
    qword_14E650FE8 = (__int64)v22;
    (**v22)(v22);
    v20 = qword_14E650FE8;
  }
  sub_144361990(v20, 1, &v43);
  v23 = qword_14E650FE8;
  if ( !qword_14E650FE8 )
  {
    v24 = sub_146E8BA20(96);
    v48 = v24;
    if ( v24 )
      v25 = (void (__fastcall ***)(_QWORD))sub_14435F9D0(v24);
    else
      v25 = 0;
    qword_14E650FE8 = (__int64)v25;
    (**v25)(v25);
    v23 = qword_14E650FE8;
  }
  v26 = sub_145F0BA60(qword_14E683C08);
  sub_144364430(v23, v26);
  v27 = sub_14667BB90(qword_14E683C78, 578, 0);
  v28 = sub_148AA307C(v27, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DD25450, 0);
  if ( v28 )
    sub_144192260(v28);
  if ( !qword_14E638F28 )
  {
    v29 = sub_146E8BA20(1472);
    v48 = v29;
    if ( v29 )
      v30 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v29);
    else
      v30 = 0;
    qword_14E638F28 = (__int64)v30;
    (**v30)(v30);
  }
  result = sub_1444EC440();
  if ( (_BYTE)result )
  {
    v32 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v33 = sub_146E8BA20(1472);
      v48 = v33;
      if ( v33 )
        v34 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v33);
      else
        v34 = 0;
      qword_14E638F28 = (__int64)v34;
      (**v34)(v34);
      v32 = qword_14E638F28;
    }
    result = sub_1444F1EE0(v32, 0);
  }
  v35 = v45;
  if ( (_QWORD)v45 )
  {
    v36 = 24 * ((v46 - (__int64)v45) / 24);
    if ( v36 >= 0x1000 )
    {
      v36 += 39LL;
      v35 = *(_QWORD *)(v45 - 8);
      if ( (unsigned __int64)(v45 - v35 - 8) > 0x1F )
        sub_148AAF304(v46 - v45, v36);
    }
    result = sub_146E9F3A0(v35, v36);
    v45 = 0;
    v46 = 0;
  }
  v37 = v43;
  if ( (_QWORD)v43 )
  {
    v38 = 24 * ((v44 - (__int64)v43) / 24);
    if ( v38 >= 0x1000 )
    {
      v38 += 39LL;
      v37 = *(_QWORD *)(v43 - 8);
      if ( (unsigned __int64)(v43 - v37 - 8) > 0x1F )
        sub_148AAF304(v44 - v43, v38);
    }
    result = sub_146E9F3A0(v37, v38);
    v43 = 0;
    v44 = 0;
  }
  return result;
}
