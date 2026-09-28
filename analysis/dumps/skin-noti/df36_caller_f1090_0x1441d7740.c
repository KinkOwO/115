// caller_f1090_0x1441d7740

__int64 __fastcall sub_1441D7740(__int64 *a1, __int64 *a2, __int64 a3)
{
  __int64 *v3; // rdi
  __int64 v5; // rax
  __int64 v6; // rax
  __int64 v7; // rbx
  unsigned int *i; // r14
  __int64 v9; // rax
  __int64 v10; // rax
  int v11; // eax
  int v12; // eax
  __int64 v13; // r13
  __int64 v14; // rax
  void (__fastcall ***v15)(_QWORD); // rcx
  __int64 v16; // rsi
  __int64 v17; // r12
  __int64 v18; // rbx
  __int64 v19; // rdi
  __int64 v20; // rbx
  __int64 v21; // rcx
  unsigned __int64 v22; // rdx
  __int64 v23; // rax
  __int64 v24; // r12
  __int64 v25; // rax
  void (__fastcall ***v26)(_QWORD); // rcx
  __int64 v27; // rsi
  __int64 v28; // r13
  __int64 v29; // rbx
  __int64 v30; // rdi
  __int64 v31; // rbx
  __int64 v32; // rax
  __int128 v34; // [rsp+40h] [rbp-71h] BYREF
  __int64 v35; // [rsp+50h] [rbp-61h]
  __int64 v36; // [rsp+58h] [rbp-59h]
  __int128 v37; // [rsp+60h] [rbp-51h] BYREF
  __int64 v38; // [rsp+70h] [rbp-41h]
  __int128 v39; // [rsp+78h] [rbp-39h] BYREF
  __int64 v40; // [rsp+88h] [rbp-29h]
  __int128 *v41; // [rsp+90h] [rbp-21h]
  __int128 *v42; // [rsp+98h] [rbp-19h]
  __int64 v43; // [rsp+A0h] [rbp-11h]
  __int64 v44; // [rsp+A8h] [rbp-9h]
  __int128 *v45; // [rsp+B0h] [rbp-1h]
  __int64 v46; // [rsp+B8h] [rbp+7h]
  __int128 *v47; // [rsp+C0h] [rbp+Fh]
  __int64 *v48; // [rsp+118h] [rbp+67h]
  int v49; // [rsp+120h] [rbp+6Fh] BYREF

  v48 = a2;
  v43 = -2;
  v3 = a2;
  if ( (_DWORD)a3 == 13 )
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
    v36 = 0;
    for ( i = (unsigned int *)(a1 + 33); ; i += 104 )
    {
      v9 = sub_1444EBAB0(*i, (__int64)a2, a3);
      if ( v9 && *(_DWORD *)(v9 + 8) == 8 )
      {
        v10 = *v3;
        if ( *v3 == *((_QWORD *)i + 1) )
        {
          sub_1441E0820(a1[1], 8, *i, 0);
          (*(void (__fastcall **)(__int64 *, _QWORD))(*a1 + 32))(a1, 0);
          v11 = sub_146E8C7D0(&unk_149242AE8);
          sub_145A31380(v11, -1, 0, 0, -1, -1, 0);
        }
        else if ( v10 == *((_QWORD *)i + 23) )
        {
          sub_1441E0820(a1[1], 8, *i, 0);
          v12 = sub_146E8C7D0(&unk_149242AE8);
          sub_145A31380(v12, -1, 0, 0, -1, -1, 0);
          v37 = 0;
          v38 = 0;
          sub_140154010(&v37, 0, i);
          v13 = qword_14E638F28;
          if ( !qword_14E638F28 )
          {
            v14 = sub_146E8BA20(1472);
            v44 = v14;
            if ( v14 )
              v15 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v14);
            else
              v15 = 0;
            qword_14E638F28 = (__int64)v15;
            (**v15)(v15);
            v13 = qword_14E638F28;
          }
          v45 = &v34;
          v34 = 0;
          v35 = 0;
          v16 = v37;
          if ( (_QWORD)v37 != *((_QWORD *)&v37 + 1) )
          {
            v17 = *((_QWORD *)&v37 + 1) - v37;
            v18 = (__int64)(*((_QWORD *)&v37 + 1) - v37) >> 2;
            *(_QWORD *)&v34 = sub_140157580(&v34, v18);
            *((_QWORD *)&v34 + 1) = v34;
            v19 = 4 * v18;
            v35 = 4 * v18 + v34;
            v41 = &v34;
            v20 = v34;
            sub_148AA1E60(v34, v16, v17);
            *((_QWORD *)&v34 + 1) = v20 + v19;
            v41 = 0;
            v7 = v36;
            v3 = v48;
          }
          sub_1444F1090(v13, 8, &v34);
          if ( v16 )
          {
            v22 = (v38 - v16) & 0xFFFFFFFFFFFFFFFCuLL;
            v23 = v16;
            if ( v22 >= 0x1000 )
            {
              v22 += 39LL;
              v16 = *(_QWORD *)(v16 - 8);
              if ( (unsigned __int64)(v23 - v16 - 8) > 0x1F )
                goto LABEL_46;
            }
            sub_146E9F3A0(v16, v22);
            v37 = 0;
            v38 = 0;
          }
        }
        else if ( v10 == *((_QWORD *)i + 25) )
        {
          v49 = 200000;
          v39 = 0;
          v40 = 0;
          sub_140154010(&v39, 0, &v49);
          v24 = qword_14E638F28;
          if ( !qword_14E638F28 )
          {
            v25 = sub_146E8BA20(1472);
            v46 = v25;
            if ( v25 )
              v26 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v25);
            else
              v26 = 0;
            qword_14E638F28 = (__int64)v26;
            (**v26)(v26);
            v24 = qword_14E638F28;
          }
          v47 = &v34;
          v34 = 0;
          v35 = 0;
          v27 = v39;
          if ( (_QWORD)v39 != *((_QWORD *)&v39 + 1) )
          {
            v28 = *((_QWORD *)&v39 + 1) - v39;
            v29 = (__int64)(*((_QWORD *)&v39 + 1) - v39) >> 2;
            *(_QWORD *)&v34 = sub_140157580(&v34, v29);
            *((_QWORD *)&v34 + 1) = v34;
            v30 = 4 * v29;
            v35 = 4 * v29 + v34;
            v42 = &v34;
            v31 = v34;
            sub_148AA1E60(v34, v27, v28);
            *((_QWORD *)&v34 + 1) = v31 + v30;
            v42 = 0;
            v7 = v36;
            v3 = v48;
          }
          sub_1444F1090(v24, 8, &v34);
          if ( v27 )
          {
            v22 = (v40 - v27) & 0xFFFFFFFFFFFFFFFCuLL;
            v32 = v27;
            if ( v22 >= 0x1000 )
            {
              v22 += 39LL;
              v27 = *(_QWORD *)(v27 - 8);
              if ( (unsigned __int64)(v32 - v27 - 8) > 0x1F )
LABEL_46:
                sub_148AAF304(v21, v22);
            }
            sub_146E9F3A0(v27, v22);
            v39 = 0;
            v40 = 0;
          }
        }
      }
      v36 = ++v7;
      if ( v7 >= 3 )
        return 0;
    }
  }
  if ( (_DWORD)a3 == 24 )
  {
    if ( *a2 == a1[14] )
    {
      a2 = 0;
LABEL_44:
      (*(void (__fastcall **)(__int64 *, __int64 *))(*a1 + 32))(a1, a2);
      return 0;
    }
    if ( *a2 == a1[5] )
    {
      LOBYTE(a2) = 1;
      goto LABEL_44;
    }
  }
  return 0;
}

