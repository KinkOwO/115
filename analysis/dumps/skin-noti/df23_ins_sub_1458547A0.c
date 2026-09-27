// ins_sub_1458547A0

void __fastcall sub_1458547A0(_QWORD *a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  int v5; // r12d
  unsigned int i; // r14d
  __int64 v7; // rcx
  __int64 v8; // rax
  void (__fastcall ***v9)(_QWORD); // rcx
  __int64 v10; // rax
  __int64 v11; // r8
  _DWORD *v12; // rsi
  __int64 v13; // r15
  __int64 *v14; // r9
  __int64 *v15; // rcx
  __int64 *v16; // rdx
  __int64 v17; // rax
  __int64 v18; // rbx
  __int64 v19; // rdi
  __int64 *v20; // rax
  volatile signed __int32 *v21; // rdi
  __int64 v22; // rcx
  __int64 v23; // rax
  void (__fastcall ***v24)(_QWORD); // rcx
  __int64 v25; // r8
  _DWORD *j; // rsi
  _DWORD *v27; // r12
  __int64 v28; // r14
  __int64 *v29; // r9
  __int64 *v30; // rcx
  __int64 *v31; // rdx
  __int64 v32; // rax
  __int64 v33; // rbx
  __int64 v34; // rdi
  __int64 v35; // rdx
  __int64 v36; // r15
  __int64 v37; // rax
  __int64 v38; // rcx
  __int64 v39; // rax
  volatile signed __int32 *v40; // rdi
  __int64 v41; // rcx
  unsigned __int64 v42; // rdx
  unsigned int v43; // [rsp+20h] [rbp-79h]
  int v44; // [rsp+20h] [rbp-79h]
  int v45; // [rsp+20h] [rbp-79h]
  __int128 v46; // [rsp+58h] [rbp-41h] BYREF
  __int64 v47; // [rsp+68h] [rbp-31h]
  __int64 v48; // [rsp+70h] [rbp-29h]
  __int64 v49; // [rsp+78h] [rbp-21h]
  __int64 v50; // [rsp+80h] [rbp-19h]
  __int64 v51; // [rsp+88h] [rbp-11h]
  __int128 v52; // [rsp+90h] [rbp-9h]
  __int64 v53; // [rsp+A8h] [rbp+Fh]
  __int128 v54[4]; // [rsp+B0h] [rbp+17h] BYREF

  v53 = -2;
  v43 = 0;
  if ( !a1[46] )
    sub_145856990();
  v2 = qword_14E64CE60;
  if ( !qword_14E64CE60 )
  {
    v3 = sub_146E8BA20(120);
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_1450AF890(v3);
    else
      v4 = 0;
    qword_14E64CE60 = (__int64)v4;
    (**v4)(v4);
    v2 = qword_14E64CE60;
  }
  v5 = sub_14017AF00(v2);
  for ( i = 0; (int)i < v5; ++i )
  {
    v7 = qword_14E64CE60;
    if ( !qword_14E64CE60 )
    {
      v8 = sub_146E8BA20(120);
      if ( v8 )
        v9 = (void (__fastcall ***)(_QWORD))sub_1450AF890(v8);
      else
        v9 = 0;
      qword_14E64CE60 = (__int64)v9;
      (**v9)(v9);
      v7 = qword_14E64CE60;
    }
    v10 = sub_1450B0810(v7, i);
    if ( v10 )
    {
      v12 = (_DWORD *)(v10 + 4);
      LOBYTE(v11) = 1;
      v13 = sub_140283D60(qword_14E683B38, *(unsigned int *)(v10 + 4), v11);
      if ( v13 )
      {
        v14 = (__int64 *)a1[87];
        v15 = (__int64 *)v14[1];
        v16 = v14;
        while ( !*((_BYTE *)v15 + 25) )
        {
          if ( *((_DWORD *)v15 + 7) >= *v12 )
          {
            v16 = v15;
            v15 = (__int64 *)*v15;
          }
          else
          {
            v15 = (__int64 *)v15[2];
          }
        }
        if ( *((_BYTE *)v16 + 25) || *v12 < *((_DWORD *)v16 + 7) || v16 == v14 )
        {
          v17 = sub_146E8BA20(264);
          v18 = v17;
          if ( v17 )
          {
            *(_OWORD *)v17 = 0;
            *(_DWORD *)(v17 + 8) = 1;
            *(_DWORD *)(v17 + 12) = 1;
            *(_QWORD *)v17 = off_1496B1738;
            sub_14586BD00(v17 + 16);
          }
          else
          {
            v18 = 0;
          }
          v44 = v43 | 1;
          v19 = v18 + 16;
          v48 = v18 + 16;
          v49 = v18;
          sub_146E8D740(v18 + 32, v13 + 496);
          *(_DWORD *)(v18 + 16) = *v12;
          *(_DWORD *)(v18 + 20) = sub_14586CEA0(*(unsigned int *)(v13 + 20));
          *(_BYTE *)(v18 + 176) = *(_BYTE *)(v13 + 5922);
          v20 = (__int64 *)sub_140457BE0(a1 + 4, v12);
          if ( v18 )
          {
            _InterlockedIncrement((volatile signed __int32 *)(v18 + 8));
            v19 = v48;
          }
          *v20 = v19;
          v21 = (volatile signed __int32 *)v20[1];
          v20[1] = v18;
          if ( v21 )
          {
            if ( _InterlockedExchangeAdd(v21 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v21)(v21);
              if ( _InterlockedExchangeAdd(v21 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v21 + 8LL))(v21);
            }
          }
          v43 = v44 & 0xFFFFFFFE;
          if ( v18 )
          {
            if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v18 + 8), 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(__int64))v18)(v18);
              if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v18 + 12), 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(__int64))(*(_QWORD *)v18 + 8LL))(v18);
            }
          }
        }
      }
    }
  }
  v22 = qword_14E64CE60;
  if ( !qword_14E64CE60 )
  {
    v23 = sub_146E8BA20(120);
    if ( v23 )
      v24 = (void (__fastcall ***)(_QWORD))sub_1450AF890(v23);
    else
      v24 = 0;
    qword_14E64CE60 = (__int64)v24;
    (**v24)(v24);
    v22 = qword_14E64CE60;
  }
  sub_1450B0660(v22, &v46);
  v27 = (_DWORD *)*((_QWORD *)&v46 + 1);
  for ( j = (_DWORD *)v46; j != v27; ++j )
  {
    LOBYTE(v25) = 1;
    v28 = sub_140283D60(qword_14E683B38, (unsigned int)*j, v25);
    if ( v28 )
    {
      v29 = (__int64 *)a1[87];
      v30 = (__int64 *)v29[1];
      v31 = v29;
      if ( !*((_BYTE *)v30 + 25) )
      {
        v25 = (unsigned int)*j;
        do
        {
          if ( *((_DWORD *)v30 + 7) >= (int)v25 )
          {
            v31 = v30;
            v30 = (__int64 *)*v30;
          }
          else
          {
            v30 = (__int64 *)v30[2];
          }
        }
        while ( !*((_BYTE *)v30 + 25) );
      }
      if ( *((_BYTE *)v31 + 25) || *j < *((_DWORD *)v31 + 7) || v31 == v29 )
      {
        v32 = sub_146E8BA20(264);
        v33 = v32;
        if ( v32 )
        {
          *(_OWORD *)v32 = 0;
          *(_DWORD *)(v32 + 8) = 1;
          *(_DWORD *)(v32 + 12) = 1;
          *(_QWORD *)v32 = off_1496B1738;
          sub_14586BD00(v32 + 16);
        }
        else
        {
          v33 = 0;
        }
        v45 = v43 | 2;
        v34 = v33 + 16;
        v50 = v33 + 16;
        v51 = v33;
        sub_146E8D740(v33 + 32, v28 + 496);
        *(_DWORD *)(v33 + 16) = *j;
        *(_DWORD *)(v33 + 20) = sub_14586CEA0(*(unsigned int *)(v28 + 20));
        v36 = a1[4];
        v37 = *(_QWORD *)(v36 + 8);
        *(_QWORD *)&v52 = v37;
        v25 = 0;
        DWORD2(v52) = 0;
        v38 = v36;
        if ( !*(_BYTE *)(v37 + 25) )
        {
          v35 = (unsigned int)*j;
          do
          {
            *(_QWORD *)&v52 = v37;
            if ( *(_DWORD *)(v37 + 32) >= (int)v35 )
            {
              DWORD2(v52) = 1;
              v38 = v37;
              v37 = *(_QWORD *)v37;
            }
            else
            {
              DWORD2(v52) = 0;
              v37 = *(_QWORD *)(v37 + 16);
            }
          }
          while ( !*(_BYTE *)(v37 + 25) );
        }
        if ( *(_BYTE *)(v38 + 25) || *j < *(_DWORD *)(v38 + 32) )
        {
          if ( a1[5] == 0x492492492492492LL )
            sub_14014F360(v38, v35);
          v39 = sub_146E8BA20(56);
          *(_DWORD *)(v39 + 32) = *j;
          *(_QWORD *)(v39 + 40) = 0;
          *(_QWORD *)(v39 + 48) = 0;
          *(_QWORD *)v39 = v36;
          *(_QWORD *)(v39 + 8) = v36;
          *(_QWORD *)(v39 + 16) = v36;
          *(_WORD *)(v39 + 24) = 0;
          v54[0] = v52;
          v38 = sub_14014F0E0(a1 + 4, v54, v39);
        }
        if ( v33 )
        {
          _InterlockedIncrement((volatile signed __int32 *)(v33 + 8));
          v34 = v50;
        }
        *(_QWORD *)(v38 + 40) = v34;
        v40 = *(volatile signed __int32 **)(v38 + 48);
        *(_QWORD *)(v38 + 48) = v33;
        if ( v40 )
        {
          if ( _InterlockedExchangeAdd(v40 + 2, 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v40)(v40);
            if ( _InterlockedExchangeAdd(v40 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v40 + 8LL))(v40);
          }
        }
        v43 = v45 & 0xFFFFFFFD;
        if ( v33 )
        {
          if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v33 + 8), 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(__int64))v33)(v33);
            if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v33 + 12), 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(__int64))(*(_QWORD *)v33 + 8LL))(v33);
          }
        }
      }
    }
  }
  v41 = v46;
  if ( (_QWORD)v46 )
  {
    v42 = (v47 - v46) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v42 >= 0x1000 )
    {
      v42 += 39LL;
      v41 = *(_QWORD *)(v46 - 8);
      if ( (unsigned __int64)(v46 - v41 - 8) > 0x1F )
        sub_148AAF304(v41, v42);
    }
    sub_146E9F3A0(v41, v42);
    v46 = 0;
    v47 = 0;
  }
}

