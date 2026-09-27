// holder_sub_142CECF30_0x142cecf30

void __fastcall sub_142CECF30(__int64 a1, char a2)
{
  int v4; // eax
  __int64 v5; // rdx
  _QWORD *i; // rbx
  _QWORD *v7; // rbp
  __int64 v8; // rdi
  int v9; // ecx
  __int64 v10; // rax
  __int64 v11; // rdi
  __int64 v12; // rax
  __int64 v13; // rcx
  __int64 v14; // rax
  __int64 v15; // rcx
  __int64 v16; // rax
  __int64 v17; // rdx
  __int64 v18; // r8
  __int64 v19; // rcx
  __int64 v20; // rax
  void (__fastcall ***v21)(_QWORD); // rcx
  __int64 v22; // rcx
  unsigned __int64 v23; // rdx
  _QWORD *v24; // rdi
  _QWORD *j; // rbx
  __int64 v26; // rax
  __int64 v27; // rax
  __int64 v28; // rbp
  __int64 v29; // rax
  __int64 v30; // r14
  __int64 v31; // rax
  __int64 v32; // r14
  __int64 v33; // rax
  _BYTE v34[16]; // [rsp+38h] [rbp-70h] BYREF
  __int128 v35; // [rsp+48h] [rbp-60h] BYREF
  __int64 v36; // [rsp+58h] [rbp-50h]
  _BYTE v37[8]; // [rsp+60h] [rbp-48h] BYREF
  __int64 v38; // [rsp+68h] [rbp-40h]

  if ( (unsigned __int8)sub_145388560(a1) )
  {
    if ( a2 )
    {
      sub_1402E0390(a1 + 928);
      v4 = sub_145385ED0(a1);
      v35 = 0u;
      v36 = 0;
      sub_145DF44E0(v4, (unsigned int)&v35, 0, 0, 0);
      v7 = (_QWORD *)*((_QWORD *)&v35 + 1);
      for ( i = (_QWORD *)v35; i != v7; ++i )
      {
        v8 = *i;
        v9 = *(_DWORD *)(*i + 348LL);
        if ( (v9 & 0x211) != 0x211
          && ((v9 & 1) == 0
           || !(*(__int64 (__fastcall **)(_QWORD))(*(_QWORD *)v8 + 1312LL))(*i)
           || (*(_DWORD *)((*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v8 + 1312LL))(v8) + 348) & 0x211) != 0x211) )
        {
          if ( (*(_BYTE *)(*i + 348LL) & 1) != 0 )
            goto LABEL_18;
          if ( (unsigned __int8)sub_145B8D6B0(*i) && !(unsigned __int8)sub_145B8E3F0(*i) )
          {
            v10 = *(_QWORD *)(a1 + 160);
            if ( v10 && *(_DWORD *)(v10 + 8) )
              v11 = *(_QWORD *)(a1 + 168);
            else
              v11 = 0;
            v12 = (*(__int64 (__fastcall **)(_QWORD))(*(_QWORD *)*i + 1616LL))(*i);
            v13 = v11 - 48;
            if ( !v11 )
              v13 = 0;
            if ( v12 != v13 )
            {
LABEL_18:
              v14 = *(_QWORD *)(a1 + 160);
              if ( v14 && *(_DWORD *)(v14 + 8) )
                v15 = *(_QWORD *)(a1 + 168);
              else
                v15 = 0;
              v5 = *i;
              v16 = v15 - 48;
              if ( !v15 )
                v16 = 0;
              if ( v5 != v16
                && (*(unsigned __int8 (__fastcall **)(_QWORD))(*(_QWORD *)v5 + 1552LL))(*i)
                && !(unsigned __int8)sub_145B8B020(*i) )
              {
                v17 = *i + 48LL;
                if ( !*i )
                  v17 = 0;
                sub_146EA47F0(v37, v17);
                sub_1405B2BF0(a1 + 928, v34, v37);
                v19 = v38;
                if ( v38 && _InterlockedExchangeAdd((volatile signed __int32 *)(v38 + 12), 0xFFFFFFFF) == 1 )
                  (*(void (__fastcall **)(__int64))(*(_QWORD *)v19 + 8LL))(v19);
                LOBYTE(v18) = 1;
                sub_142CED340(a1, *i, v18);
              }
            }
          }
        }
      }
      LOBYTE(v5) = 1;
      sub_142CECBF0(a1, v5);
      if ( !qword_14E63AE60 )
      {
        v20 = sub_146E8BA20(336);
        if ( v20 )
          v21 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v20);
        else
          v21 = 0;
        qword_14E63AE60 = (__int64)v21;
        (**v21)(v21);
      }
      sub_1447E6A00();
      v22 = v35;
      if ( (_QWORD)v35 )
      {
        v23 = (v36 - v35) & 0xFFFFFFFFFFFFFFF8uLL;
        if ( v23 >= 0x1000 )
        {
          v23 += 39LL;
          v22 = *(_QWORD *)(v35 - 8);
          if ( (unsigned __int64)(v35 - v22 - 8) > 0x1F )
            sub_148AAF304(v22, v23);
        }
        sub_146E9F3A0(v22, v23);
        v35 = 0;
        v36 = 0;
      }
    }
    else
    {
      v24 = *(_QWORD **)(a1 + 936);
      for ( j = (_QWORD *)*v24; j != v24; j = (_QWORD *)*j )
      {
        v26 = j[3];
        if ( v26 && *(_DWORD *)(v26 + 8) )
          v27 = j[4];
        else
          v27 = 0;
        v28 = v27 - 48;
        if ( !v27 )
          v28 = 0;
        if ( (unsigned __int8)sub_145388560(a1) && v28 )
        {
          sub_145B97380(v28, 0);
          v29 = sub_1450BE260(v28);
          v30 = v29;
          if ( v29 )
          {
            sub_145C3AAF0(v29, 0);
            (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v30 + 2648LL))(v30, 0);
          }
          v31 = sub_1450BE2C0(v28);
          v32 = v31;
          if ( v31 && (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v31 + 6048LL))(v31) )
          {
            v33 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v32 + 6048LL))(v32);
            sub_145E4C210(v33, 0);
          }
        }
      }
      sub_1402E0390(a1 + 928);
      if ( (unsigned __int8)sub_145388560(a1) )
      {
        if ( *(float *)(a1 + 996) >= 0.0 )
        {
          sub_146B35C50();
          *(_DWORD *)(a1 + 996) = -1082130432;
        }
        if ( *(int *)(a1 + 1000) >= 0 )
        {
          sub_1447EF1D0();
          *(_DWORD *)(a1 + 1000) = -1;
        }
      }
    }
    *(_BYTE *)(a1 + 992) = a2;
  }
}

