// dfvtable_5_1441D4E20

__int64 __fastcall sub_1441D4E20(_QWORD *a1, __int64 *a2, __int64 a3)
{
  __int64 *v3; // rdi
  _QWORD *v4; // rsi
  __int64 v5; // rax
  unsigned int v6; // r12d
  __int64 v7; // rax
  __int64 v8; // rbx
  unsigned int *v9; // r15
  __int64 v10; // rax
  __int64 v11; // rax
  int v12; // eax
  int v13; // eax
  __int64 v14; // rax
  void (__fastcall ***v15)(_QWORD); // rcx
  __int64 v16; // r14
  __int64 v17; // r13
  __int64 v18; // rbx
  __int64 v19; // rdi
  __int64 v20; // rbx
  __int64 v21; // rcx
  unsigned __int64 v22; // rdx
  __int64 v23; // rax
  int v24; // eax
  __int64 v25; // rcx
  __int64 v26; // rax
  void (__fastcall ***v27)(_QWORD); // rcx
  __int64 v28; // rax
  __int64 v29; // rax
  __int128 v31; // [rsp+40h] [rbp-61h] BYREF
  __int64 v32; // [rsp+50h] [rbp-51h]
  __int64 v33; // [rsp+58h] [rbp-49h]
  __int128 v34; // [rsp+60h] [rbp-41h] BYREF
  __int64 v35; // [rsp+70h] [rbp-31h]
  __int128 *v36; // [rsp+78h] [rbp-29h]
  __int64 v37; // [rsp+80h] [rbp-21h]
  __int64 v38; // [rsp+88h] [rbp-19h]
  __int128 *v39; // [rsp+90h] [rbp-11h]
  __int64 v40; // [rsp+98h] [rbp-9h]
  __int128 *v41; // [rsp+A0h] [rbp-1h]
  __int128 v42; // [rsp+A8h] [rbp+7h]
  __int64 v43; // [rsp+B8h] [rbp+17h]
  __int64 v44; // [rsp+100h] [rbp+5Fh]
  __int64 *v45; // [rsp+108h] [rbp+67h]

  v45 = a2;
  v37 = -2;
  v3 = a2;
  v4 = a1;
  if ( (_DWORD)a3 == 13 )
  {
    v5 = a1[9];
    if ( v5 && v5 == *a2 )
    {
      (*(void (__fastcall **)(_QWORD *))(*a1 + 112LL))(a1);
      goto LABEL_5;
    }
    v7 = a1[11];
    if ( v7 && v7 == *a2 )
    {
      (*(void (__fastcall **)(_QWORD *))(*a1 + 104LL))(a1);
LABEL_5:
      v6 = 6;
      if ( !*((_DWORD *)v4 + 546) )
        v6 = 2;
LABEL_14:
      v8 = 0;
      v33 = 0;
      v9 = (unsigned int *)(v4 + 24);
      do
      {
        v10 = sub_1444EBAB0(*v9, (__int64)a2, a3);
        if ( v10 && *(_DWORD *)(v10 + 8) == 2 )
        {
          v11 = *v3;
          if ( *v3 == *((_QWORD *)v9 + 1) )
          {
            sub_1441E0820(v4[1], v6, *v9, 0);
            (*(void (__fastcall **)(_QWORD *, _QWORD))(*v4 + 32LL))(v4, 0);
            v12 = sub_146E8C7D0(&unk_149242AE8);
            sub_145A31380(v12, -1, 0, 0, -1, -1, 0);
          }
          else if ( v11 == *((_QWORD *)v9 + 26) )
          {
            sub_1441E0820(v4[1], v6, *v9, 0);
            v13 = sub_146E8C7D0(&unk_149242AE8);
            sub_145A31380(v13, -1, 0, 0, -1, -1, 0);
            v34 = 0;
            v35 = 0;
            sub_140154010(&v34, 0, v9);
            v44 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v14 = sub_146E8BA20(1472);
              v38 = v14;
              if ( v14 )
                v15 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v14);
              else
                v15 = 0;
              qword_14E638F28 = (__int64)v15;
              (**v15)(v15);
              v44 = qword_14E638F28;
            }
            v39 = &v31;
            v31 = 0;
            v32 = 0;
            v16 = v34;
            if ( (_QWORD)v34 != *((_QWORD *)&v34 + 1) )
            {
              v17 = *((_QWORD *)&v34 + 1) - v34;
              v18 = (__int64)(*((_QWORD *)&v34 + 1) - v34) >> 2;
              *(_QWORD *)&v31 = sub_140157580(&v31, v18);
              *((_QWORD *)&v31 + 1) = v31;
              v19 = 4 * v18;
              v32 = 4 * v18 + v31;
              v36 = &v31;
              v20 = v31;
              sub_148AA1E60(v31, v16, v17);
              *((_QWORD *)&v31 + 1) = v20 + v19;
              v36 = 0;
              v8 = v33;
              v3 = v45;
            }
            sub_1444F1090(v44, v6, &v31);
            if ( v16 )
            {
              v22 = (v35 - v16) & 0xFFFFFFFFFFFFFFFCuLL;
              v23 = v16;
              if ( v22 >= 0x1000 )
              {
                v22 += 39LL;
                v16 = *(_QWORD *)(v16 - 8);
                if ( (unsigned __int64)(v23 - v16 - 8) > 0x1F )
                  sub_148AAF304(v21, v22);
              }
              sub_146E9F3A0(v16, v22);
              v34 = 0;
              v35 = 0;
            }
          }
          else if ( v11 == *((_QWORD *)v9 + 28) )
          {
            v24 = sub_146E8C7D0(&unk_149242AE8);
            sub_145A31380(v24, -1, 0, 0, -1, -1, 0);
            v42 = 0;
            v43 = 0;
            v25 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v26 = sub_146E8BA20(1472);
              v40 = v26;
              if ( v26 )
                v27 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v26);
              else
                v27 = 0;
              qword_14E638F28 = (__int64)v27;
              (**v27)(v27);
              v25 = qword_14E638F28;
            }
            v41 = &v31;
            v31 = 0;
            v32 = 0;
            sub_1444F1090(v25, v6, &v31);
          }
        }
        v33 = ++v8;
        v9 += 100;
      }
      while ( v8 < 5 );
      return 0;
    }
  }
  v6 = 6;
  if ( !*((_DWORD *)a1 + 546) )
    v6 = 2;
  if ( (_DWORD)a3 == 13 )
    goto LABEL_14;
  if ( (_DWORD)a3 == 24 )
  {
    v28 = *a2;
    if ( *a2 == a1[14] )
    {
      v29 = *a1;
      a2 = 0;
    }
    else
    {
      if ( v28 == a1[5] )
      {
        v29 = *a1;
      }
      else
      {
        if ( v28 != a1[274] )
          return 0;
        (*(void (__fastcall **)(_QWORD *))(*a1 + 104LL))(a1);
        *((_DWORD *)v4 + 32) = -1;
        *((_DWORD *)v4 + 546) = sub_146F50350(v4[274]);
        (*(void (__fastcall **)(_QWORD *))(*v4 + 24LL))(v4);
        v29 = *v4;
        a1 = v4;
      }
      LOBYTE(a2) = 1;
    }
    (*(void (__fastcall **)(_QWORD *, __int64 *))(v29 + 32))(a1, a2);
  }
  return 0;
}

