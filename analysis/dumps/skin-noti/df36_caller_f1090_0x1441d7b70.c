// caller_f1090_0x1441d7b70

__int64 __fastcall sub_1441D7B70(__int64 *a1, _QWORD *a2, int a3)
{
  __int64 v4; // rdi
  _QWORD *v5; // r15
  __int64 v6; // rax
  __int64 v7; // r13
  __int64 v8; // rax
  void (__fastcall ***v9)(_QWORD); // rcx
  __int64 v10; // r14
  __int64 v11; // r12
  __int64 v12; // rbx
  __int64 v13; // rdi
  __int64 v14; // rbx
  int v15; // eax
  __int64 v16; // rcx
  unsigned __int64 v17; // rdx
  __int64 v18; // rax
  __int64 v19; // r13
  __int64 v20; // rax
  void (__fastcall ***v21)(_QWORD); // rcx
  __int64 v22; // r14
  __int64 v23; // r12
  __int64 v24; // rbx
  __int64 v25; // rdi
  __int64 v26; // rbx
  int v27; // eax
  __int64 v28; // rax
  __int128 v30; // [rsp+40h] [rbp-71h] BYREF
  __int64 v31; // [rsp+50h] [rbp-61h]
  __int64 v32; // [rsp+58h] [rbp-59h]
  __int128 v33; // [rsp+60h] [rbp-51h] BYREF
  __int64 v34; // [rsp+70h] [rbp-41h]
  __int128 v35; // [rsp+78h] [rbp-39h] BYREF
  __int64 v36; // [rsp+88h] [rbp-29h]
  __int128 *v37; // [rsp+90h] [rbp-21h]
  __int128 *v38; // [rsp+98h] [rbp-19h]
  __int64 v39; // [rsp+A0h] [rbp-11h]
  __int64 v40; // [rsp+A8h] [rbp-9h]
  __int128 *v41; // [rsp+B0h] [rbp-1h]
  __int64 v42; // [rsp+B8h] [rbp+7h]
  __int128 *v43; // [rsp+C0h] [rbp+Fh]
  _QWORD *v44; // [rsp+118h] [rbp+67h]
  int v45; // [rsp+120h] [rbp+6Fh] BYREF

  v44 = a2;
  v39 = -2;
  if ( a3 == 13 )
  {
    v4 = 0;
    v32 = 0;
    v5 = a1 + 25;
    while ( 1 )
    {
      v6 = *a2;
      if ( *a2 == v5[8] || v6 == *v5 )
        break;
      if ( v6 == v5[22] )
      {
        sub_1441E0820(a1[1], 4, *((unsigned int *)v5 - 2), 0);
        (*(void (__fastcall **)(__int64 *, _QWORD))(*a1 + 32))(a1, 0);
        v33 = 0;
        v34 = 0;
        sub_140154010(&v33, 0, v5 - 1);
        v7 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          v8 = sub_146E8BA20(1472);
          v40 = v8;
          if ( v8 )
            v9 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v8);
          else
            v9 = 0;
          qword_14E638F28 = (__int64)v9;
          (**v9)(v9);
          v7 = qword_14E638F28;
        }
        v41 = &v30;
        v30 = 0;
        v31 = 0;
        v10 = v33;
        if ( (_QWORD)v33 != *((_QWORD *)&v33 + 1) )
        {
          v11 = *((_QWORD *)&v33 + 1) - v33;
          v12 = (__int64)(*((_QWORD *)&v33 + 1) - v33) >> 2;
          *(_QWORD *)&v30 = sub_140157580(&v30, v12);
          *((_QWORD *)&v30 + 1) = v30;
          v13 = 4 * v12;
          v31 = 4 * v12 + v30;
          v37 = &v30;
          v14 = v30;
          sub_148AA1E60(v30, v10, v11);
          *((_QWORD *)&v30 + 1) = v14 + v13;
          v37 = 0;
          v4 = v32;
        }
        sub_1444F1090(v7, 4, &v30);
        (*(void (__fastcall **)(__int64 *, _QWORD))(*a1 + 32))(a1, 0);
        v15 = sub_146E8C7D0(&unk_149242AE8);
        sub_145A31380(v15, -1, 0, 0, -1, -1, 0);
        if ( v10 )
        {
          v17 = (v34 - v10) & 0xFFFFFFFFFFFFFFFCuLL;
          v18 = v10;
          if ( v17 >= 0x1000 )
          {
            v17 += 39LL;
            v10 = *(_QWORD *)(v10 - 8);
            if ( (unsigned __int64)(v18 - v10 - 8) > 0x1F )
              goto LABEL_43;
          }
          sub_146E9F3A0(v10, v17);
          v33 = 0;
          v34 = 0;
        }
        goto LABEL_30;
      }
      if ( v6 == v5[24] )
      {
        sub_1441E0820(a1[1], 4, 0, 0);
        (*(void (__fastcall **)(__int64 *, _QWORD))(*a1 + 32))(a1, 0);
        v35 = 0;
        v36 = 0;
        v45 = 0;
        sub_140154010(&v35, 0, &v45);
        v19 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          v20 = sub_146E8BA20(1472);
          v42 = v20;
          if ( v20 )
            v21 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v20);
          else
            v21 = 0;
          qword_14E638F28 = (__int64)v21;
          (**v21)(v21);
          v19 = qword_14E638F28;
        }
        v43 = &v30;
        v30 = 0;
        v31 = 0;
        v22 = v35;
        if ( (_QWORD)v35 != *((_QWORD *)&v35 + 1) )
        {
          v23 = *((_QWORD *)&v35 + 1) - v35;
          v24 = (__int64)(*((_QWORD *)&v35 + 1) - v35) >> 2;
          *(_QWORD *)&v30 = sub_140157580(&v30, v24);
          *((_QWORD *)&v30 + 1) = v30;
          v25 = 4 * v24;
          v31 = 4 * v24 + v30;
          v38 = &v30;
          v26 = v30;
          sub_148AA1E60(v30, v22, v23);
          *((_QWORD *)&v30 + 1) = v26 + v25;
          v38 = 0;
          v4 = v32;
        }
        sub_1444F1090(v19, 4, &v30);
        (*(void (__fastcall **)(__int64 *, _QWORD))(*a1 + 32))(a1, 0);
        v27 = sub_146E8C7D0(&unk_149242AE8);
        sub_145A31380(v27, -1, 0, 0, -1, -1, 0);
        if ( v22 )
        {
          v17 = (v36 - v22) & 0xFFFFFFFFFFFFFFFCuLL;
          v28 = v22;
          if ( v17 >= 0x1000 )
          {
            v17 += 39LL;
            v22 = *(_QWORD *)(v22 - 8);
            if ( (unsigned __int64)(v28 - v22 - 8) > 0x1F )
LABEL_43:
              sub_148AAF304(v16, v17);
          }
          sub_146E9F3A0(v22, v17);
          v35 = 0;
          v36 = 0;
        }
        goto LABEL_30;
      }
LABEL_31:
      v32 = ++v4;
      v5 += 40;
      if ( v4 >= 7 )
      {
        if ( *a2 == a1[9] )
          goto LABEL_35;
        if ( *a2 == a1[11] )
        {
          sub_146F21850(a1[5], 0);
          sub_146F58600(a1[7]);
          (*(void (__fastcall **)(__int64, char *))(*(_QWORD *)a1[7] + 672LL))(a1[7], &byte_14BAF7F08);
LABEL_35:
          sub_1441C41E0((__int64)a1);
        }
        return 0;
      }
    }
    sub_1441E0820(a1[1], 4, *((unsigned int *)v5 - 2), 0);
    (*(void (__fastcall **)(__int64 *, _QWORD))(*a1 + 32))(a1, 0);
LABEL_30:
    a2 = v44;
    goto LABEL_31;
  }
  if ( a3 == 24 )
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

