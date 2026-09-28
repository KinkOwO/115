// reader_sub_1441DD510

_UNKNOWN **__fastcall sub_1441DD510(__int64 a1)
{
  _UNKNOWN **result; // rax
  _QWORD *v3; // r15
  _QWORD *v4; // rdi
  __int64 v5; // rcx
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx
  int v8; // r12d
  __int64 v9; // rax
  __int64 v10; // rax
  __int64 v11; // rax
  __int64 v12; // r8
  int v13; // r14d
  _QWORD *v14; // rsi
  int v15; // ebx
  int v16; // edi
  int v17; // edx
  int v18; // r8d
  unsigned __int64 v19; // rdx
  __int64 v20; // rcx
  char v21; // bl
  int v22; // eax
  __int64 v23; // rax
  __int64 v24; // rcx
  __int64 v25; // rax
  void (__fastcall ***v26)(_QWORD); // rcx
  __int64 v27; // rax
  int v28[2]; // [rsp+68h] [rbp-59h] BYREF
  _QWORD *v29; // [rsp+70h] [rbp-51h]
  __int64 v30; // [rsp+78h] [rbp-49h]
  __int64 v31; // [rsp+80h] [rbp-41h]
  char v32[8]; // [rsp+88h] [rbp-39h] BYREF
  char v33[8]; // [rsp+90h] [rbp-31h] BYREF
  __int64 v34; // [rsp+98h] [rbp-29h]
  _BYTE v35[16]; // [rsp+A0h] [rbp-21h] BYREF
  _QWORD v36[2]; // [rsp+B0h] [rbp-11h] BYREF
  __int64 v37; // [rsp+C0h] [rbp-1h]
  unsigned __int64 v38; // [rsp+C8h] [rbp+7h]
  _UNKNOWN *retaddr; // [rsp+120h] [rbp+5Fh] BYREF

  result = &retaddr;
  v30 = -2;
  v3 = *(_QWORD **)(a1 + 3280);
  v4 = *(_QWORD **)(a1 + 3288);
  v29 = v4;
  if ( v3 == v4 )
    return result;
  while ( 1 )
  {
    if ( !*v3 || !v3[2] )
      goto LABEL_31;
    sub_1441C3DB0((_DWORD *)a1, (__int64)v28);
    v5 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v6 = sub_146E8BA20(1472);
      v31 = v6;
      if ( v6 )
        v7 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v6);
      else
        v7 = 0;
      qword_14E638F28 = (__int64)v7;
      (**v7)(v7);
      v5 = qword_14E638F28;
    }
    v8 = sub_1444EB840(v5, v28[0], v28[1]);
    if ( (unsigned __int8)sub_146ED0010(*v3) )
    {
      v9 = sub_14723C170(100010025);
      v10 = sub_146E8CF20(v35, v9, (unsigned int)v8);
      v11 = sub_14014F430(v10);
      v36[0] = 0;
      v37 = 0;
      v38 = 7;
      v12 = -1;
      do
        ++v12;
      while ( *(_WORD *)(v11 + 2 * v12) );
      sub_14014C8D0(v36, v11);
      sub_146E8C910(v35);
      v13 = sub_1429BDDE0(qword_14E683C78);
      v14 = v36;
      if ( v38 >= 8 )
        v14 = (_QWORD *)v36[0];
      v15 = dword_14F1C0788;
      v16 = dword_14F1C0880;
      sub_146EC9E00(*v3, v32);
      sub_146EC9E00(*v3, v33);
      sub_145561F90(v13, v17, v18, v16, v15, 1, (__int64)v14, 0, 0, 1153957888, 0, 0);
      if ( v38 >= 8 )
      {
        v19 = 2 * v38 + 2;
        v20 = v36[0];
        if ( v19 >= 0x1000 )
        {
          v19 = 2 * v38 + 41;
          v20 = *(_QWORD *)(v36[0] - 8LL);
          if ( (unsigned __int64)(v36[0] - v20 - 8) > 0x1F )
            sub_148AAF304(v20, v19);
        }
        sub_146E9F3A0(v20, v19);
      }
      v37 = 0;
      v38 = 7;
      LOWORD(v36[0]) = 0;
      v4 = v29;
    }
    result = (_UNKNOWN **)sub_146ECFD90(*v3);
    if ( !(_BYTE)result )
      goto LABEL_31;
    result = (_UNKNOWN **)sub_141FB6530(v3[2]);
    v21 = (char)result;
    if ( !(_BYTE)result && v8 >= 10 )
      break;
    v22 = sub_146E8C7D0(&unk_149253258);
    result = (_UNKNOWN **)sub_145A31380(v22, -1, 0, 0, -1, -1, 0);
    if ( *((_BYTE *)v3 + 40) )
    {
      if ( qword_14E683C78 )
      {
        v23 = sub_14723C170(101036910);
        result = (_UNKNOWN **)sub_14668C520(qword_14E683C78, 2875, v23, 0);
      }
    }
    else
    {
      v24 = qword_14E638F28;
      if ( !qword_14E638F28 )
      {
        v25 = sub_146E8BA20(1472);
        v34 = v25;
        if ( v25 )
          v26 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v25);
        else
          v26 = 0;
        qword_14E638F28 = (__int64)v26;
        (**v26)(v26);
        v24 = qword_14E638F28;
      }
      result = (_UNKNOWN **)sub_1444F0FE0(v24, *((_DWORD *)v3 + 9), *((_DWORD *)v3 + 8), v21);
    }
LABEL_31:
    v3 += 6;
    if ( v3 == v4 )
      return result;
  }
  if ( qword_14E683C78 )
  {
    v27 = sub_14723C170(101037008);
    return (_UNKNOWN **)sub_14668C520(qword_14E683C78, 2875, v27, 0);
  }
  return result;
}

