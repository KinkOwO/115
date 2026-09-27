// holder_sub_146350C80_0x146350c80

__int64 __fastcall sub_146350C80(__int64 a1, unsigned __int8 a2)
{
  __int64 result; // rax
  __int64 v5; // rax
  __int64 v6; // rbx
  _QWORD *v7; // rdi
  _QWORD *v8; // rbp
  __int64 v9; // rax
  __int64 v10; // rax
  __int64 v11; // rcx
  __int64 v12; // rcx
  int v13; // ebx
  __int64 v14; // rax
  __int64 v15; // rcx
  __int64 v16; // rdx
  __int64 v17; // rax
  __int64 v18; // rdx
  __int64 v19; // r8
  __int64 v20; // rcx
  _QWORD *v21; // rax
  __int64 v22; // rax
  void (__fastcall ***v23)(_QWORD); // rcx
  __int64 v24; // rcx
  unsigned __int64 v25; // rdx
  _QWORD *v26; // rdi
  _QWORD *i; // rbx
  __int64 v28; // rax
  __int64 v29; // rax
  __int64 v30; // rbp
  __int64 v31; // rax
  __int64 v32; // r14
  __int64 v33; // rax
  __int64 v34; // r14
  __int64 v35; // rax
  __int64 v36; // rax
  _BYTE v37[16]; // [rsp+38h] [rbp-80h] BYREF
  __int128 v38; // [rsp+48h] [rbp-70h] BYREF
  __int64 v39; // [rsp+58h] [rbp-60h]
  _BYTE v40[8]; // [rsp+60h] [rbp-58h] BYREF
  __int64 v41; // [rsp+68h] [rbp-50h]

  result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 1392LL))(a1);
  if ( (_BYTE)result )
  {
    v5 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 304LL))(a1);
    if ( a2 )
    {
      v6 = v5;
      if ( v5 )
        sub_145DF9270(v5);
      sub_1402E0390(a1 + 107952);
      v38 = 0u;
      v39 = 0;
      v7 = 0;
      v8 = 0;
      if ( v6 )
      {
        sub_145DF44E0(v6, (unsigned int)&v38, 0, 0, 0);
        v8 = (_QWORD *)*((_QWORD *)&v38 + 1);
        v7 = (_QWORD *)v38;
      }
      for ( ; v7 != v8; ++v7 )
      {
        if ( (*(_BYTE *)(*v7 + 348LL) & 1) != 0
          || (unsigned __int8)sub_145B8D6B0(*v7) && !(unsigned __int8)sub_145B8E3F0(*v7) )
        {
          v9 = *(_QWORD *)(a1 + 104256);
          if ( !v9 || !*(_DWORD *)(v9 + 8) )
            goto LABEL_19;
          v10 = *(_QWORD *)(a1 + 104264);
          v11 = v10 - 48;
          if ( !v10 )
            v11 = 0;
          if ( !v11 )
            goto LABEL_19;
          v12 = v10 - 48;
          if ( !v10 )
            v12 = 0;
          v13 = sub_145B8B0A0(v12);
          if ( (unsigned int)sub_145B8B0A0(*v7) != v13 )
          {
LABEL_19:
            v14 = *(_QWORD *)(a1 + 107912);
            if ( v14 && *(_DWORD *)(v14 + 8) )
              v15 = *(_QWORD *)(a1 + 107920);
            else
              v15 = 0;
            v16 = *v7;
            v17 = v15 - 48;
            if ( !v15 )
              v17 = 0;
            if ( v16 != v17
              && (*(unsigned int (__fastcall **)(_QWORD))(*(_QWORD *)v16 + 256LL))(*v7) != 109129525
              && *v7 != a1
              && (*(unsigned __int8 (__fastcall **)(_QWORD))(*(_QWORD *)*v7 + 1552LL))(*v7)
              && !(unsigned __int8)sub_145B8B020(*v7) )
            {
              v18 = *v7 + 48LL;
              if ( !*v7 )
                v18 = 0;
              sub_146EA47F0(v40, v18);
              sub_1405B2BF0(a1 + 107952, v37, v40);
              v20 = v41;
              if ( v41 && _InterlockedExchangeAdd((volatile signed __int32 *)(v41 + 12), 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(__int64))(*(_QWORD *)v20 + 8LL))(v20);
              LOBYTE(v19) = 1;
              sub_1408BD0A0(a1, *v7, v19);
            }
          }
        }
      }
      if ( (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)a1 + 1392LL))(a1) )
      {
        if ( *(float *)(a1 + 108024) < 0.0 )
        {
          v21 = (_QWORD *)sub_145B89A50(a1);
          *(float *)(a1 + 108024) = sub_146B50820(*v21);
          sub_146B35C50();
        }
        if ( *(int *)(a1 + 108028) < 0 )
        {
          *(_DWORD *)(a1 + 108028) = sub_1447EA870();
          sub_1447EF1D0(0);
        }
      }
      if ( !qword_14E63AE60 )
      {
        v22 = sub_146E8BA20(336);
        if ( v22 )
          v23 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v22);
        else
          v23 = 0;
        qword_14E63AE60 = (__int64)v23;
        (**v23)(v23);
      }
      sub_1447E6A00();
      v24 = v38;
      if ( (_QWORD)v38 )
      {
        v25 = (v39 - v38) & 0xFFFFFFFFFFFFFFF8uLL;
        if ( v25 >= 0x1000 )
        {
          v25 += 39LL;
          v24 = *(_QWORD *)(v38 - 8);
          if ( (unsigned __int64)(v38 - v24 - 8) > 0x1F )
            sub_148AAF304(v24, v25);
        }
        sub_146E9F3A0(v24, v25);
        v38 = 0;
        v39 = 0;
      }
    }
    else
    {
      if ( v5 )
        sub_145DFEA70(v5);
      v26 = *(_QWORD **)(a1 + 107960);
      for ( i = (_QWORD *)*v26; i != v26; i = (_QWORD *)*i )
      {
        v28 = i[3];
        if ( v28 && *(_DWORD *)(v28 + 8) )
        {
          v29 = i[4];
          v30 = v29 - 48;
          if ( !v29 )
            v30 = 0;
          if ( v30 )
          {
            sub_145B97380(v30, 0);
            v31 = sub_1450BE260(v30);
            v32 = v31;
            if ( v31 )
            {
              sub_145C3AAF0(v31, 0);
              (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v32 + 2648LL))(v32, 0);
            }
            v33 = sub_1450BE2C0(v30);
            v34 = v33;
            if ( v33 && (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v33 + 6048LL))(v33) )
            {
              v35 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v34 + 6048LL))(v34);
              sub_145E4C210(v35, 0);
            }
          }
        }
      }
      sub_1402E0390(a1 + 107952);
      if ( (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)a1 + 1392LL))(a1) )
      {
        if ( *(float *)(a1 + 108024) >= 0.0 )
        {
          sub_146B35C50();
          *(_DWORD *)(a1 + 108024) = -1082130432;
        }
        if ( *(int *)(a1 + 108028) >= 0 )
        {
          ((void (*)(void))sub_1447EF1D0)();
          *(_DWORD *)(a1 + 108028) = -1;
        }
      }
    }
    *(_BYTE *)(a1 + 108033) = a2;
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)a1 + 2648LL))(a1, a2);
    sub_145C3AAF0(a1, a2);
    sub_145C3E3B0(a1, a2);
    result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 6048LL))(a1);
    if ( result )
    {
      v36 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 6048LL))(a1);
      return sub_145E4C210(v36, a2);
    }
  }
  return result;
}

