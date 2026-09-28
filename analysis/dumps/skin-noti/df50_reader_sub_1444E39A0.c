// reader_sub_1444E39A0

__int64 __fastcall sub_1444E39A0(__int64 a1)
{
  __int64 result; // rax
  unsigned int v3; // edi
  unsigned int v4; // r14d
  __int64 v5; // rcx
  __int64 v6; // rcx
  __int64 v7; // rdx
  __int64 v8; // rcx
  __int64 v9; // rdx
  __int64 v10; // rdx
  __int64 v11; // rcx
  __int64 v12; // rdx
  __int64 v13; // rdx
  __int64 v14; // rcx
  int v15; // eax
  __int64 v16; // rcx
  __int64 v17; // rcx
  __int64 v18; // rdx
  __int64 v19; // rcx
  __int64 v20; // rcx
  __int64 v21; // rcx
  __int64 v22; // rdx
  __int64 v23; // rcx
  int v24; // ebx
  unsigned int v25; // edi
  unsigned int v26; // eax
  __int64 v27; // rdi
  __int64 v28; // rcx
  void (__fastcall *v29)(__int64); // rbx
  __int64 v30; // rdi
  __int64 (__fastcall *v31)(__int64); // rbx
  volatile signed __int32 *v32; // rbx
  __int64 v33; // rcx
  __int64 v34; // rdx
  __int64 v35; // rcx
  __int64 v36; // rdx
  __int64 v37; // rdx
  __int64 v38; // rcx
  __int64 v39; // [rsp+48h] [rbp-30h] BYREF
  volatile signed __int32 *v40; // [rsp+50h] [rbp-28h]

  result = *(_DWORD *)(a1 + 6740) >> 1;
  if ( (*(_DWORD *)(a1 + 6740) & 2) != 0 )
  {
    v3 = sub_146E9F840(a1 + 6736);
    v4 = sub_140193D40(a1 + 6736);
    if ( (unsigned __int8)sub_146E9FA80(a1 + 6736) )
    {
      v3 = v4;
      sub_146E9FBC0(a1 + 6736);
      v5 = *(_QWORD *)(a1 + 6584);
      if ( v5 && (unsigned __int8)sub_141FB6530(v5) )
      {
        if ( *(_BYTE *)(a1 + 6771) )
        {
          v6 = *(_QWORD *)(a1 + 6696);
          if ( v6 )
          {
            (*(void (__fastcall **)(__int64))(*(_QWORD *)v6 + 432LL))(v6);
            LOBYTE(v7) = 1;
            (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 6696) + 16LL))(*(_QWORD *)(a1 + 6696), v7);
          }
        }
        else
        {
          v8 = *(_QWORD *)(a1 + 6600);
          if ( v8 && *(_BYTE *)(a1 + 6770) )
          {
            (*(void (__fastcall **)(__int64))(*(_QWORD *)v8 + 48LL))(v8);
            LOBYTE(v9) = 1;
            (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 6600) + 16LL))(*(_QWORD *)(a1 + 6600), v9);
            LOBYTE(v10) = 1;
            sub_146AF0130(*(_QWORD *)(a1 + 6600), v10);
          }
          v11 = *(_QWORD *)(a1 + 6632);
          if ( v11 && !*(_BYTE *)(a1 + 6770) )
          {
            (*(void (__fastcall **)(__int64))(*(_QWORD *)v11 + 48LL))(v11);
            LOBYTE(v12) = 1;
            (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 6632) + 16LL))(*(_QWORD *)(a1 + 6632), v12);
            LOBYTE(v13) = 1;
            sub_146AF0130(*(_QWORD *)(a1 + 6632), v13);
          }
        }
      }
      v14 = *(_QWORD *)(a1 + 6712);
      if ( v14 )
      {
        sub_146AF0130(v14, 0);
        (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 6712) + 432LL))(*(_QWORD *)(a1 + 6712));
      }
      v15 = sub_146E8C7D0(&unk_149C78430);
      sub_145A31380(v15, -1, 0, 0, -1, -1, 0);
    }
    result = sub_146EA1750(0, 255, v3, v4);
    v16 = *(_QWORD *)(a1 + 6408);
    if ( v16 )
      result = (*(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v16 + 376LL))(v16, (unsigned int)result);
  }
  v17 = *(_QWORD *)(a1 + 6584);
  if ( v17 )
  {
    result = sub_141FB6530(v17);
    if ( (_BYTE)result )
    {
      result = sub_146AEF960(*(_QWORD *)(a1 + 6712));
      if ( (_BYTE)result )
      {
        (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 6712) + 48LL))(*(_QWORD *)(a1 + 6712));
        LOBYTE(v18) = 1;
        result = sub_146AF0130(*(_QWORD *)(a1 + 6712), v18);
        v19 = *(_QWORD *)(a1 + 6600);
        if ( v19 && *(_BYTE *)(a1 + 6770) && !*(_BYTE *)(a1 + 6771) )
          result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v19 + 432LL))(v19);
        v20 = *(_QWORD *)(a1 + 6632);
        if ( v20 && !*(_BYTE *)(a1 + 6770) && !*(_BYTE *)(a1 + 6771) )
          result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v20 + 432LL))(v20);
        v21 = *(_QWORD *)(a1 + 6680);
        if ( v21 && !*(_BYTE *)(a1 + 6771) )
        {
          (*(void (__fastcall **)(__int64))(*(_QWORD *)v21 + 432LL))(v21);
          LOBYTE(v22) = 1;
          result = (*(__int64 (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 6680) + 16LL))(
                     *(_QWORD *)(a1 + 6680),
                     v22);
        }
      }
    }
  }
  v23 = *(_QWORD *)(a1 + 6568);
  if ( v23 )
  {
    result = sub_146AEF9C0(v23);
    if ( (_BYTE)result )
    {
      result = sub_141FB6530(*(_QWORD *)(a1 + 6568));
      if ( (_BYTE)result )
      {
        result = sub_146AF0810(*(_QWORD *)(a1 + 6568), &v39);
        if ( v39 )
        {
          v24 = sub_146B33E90(v39, 0);
          v25 = sub_146B34200(v39) - v24;
          v26 = sub_146B52140(v39, 0);
          result = sub_146EA1750(0, 10, v25, v26);
          v27 = *(_QWORD *)(a1 + 16LL * *(int *)(a1 + 6728) + 6504);
          if ( v27 )
          {
            v28 = *(_QWORD *)(a1 + 6424);
            if ( v28 )
            {
              if ( *(int *)(a1 + 6728) > -1 )
              {
                v29 = *(void (__fastcall **)(__int64))(*(_QWORD *)v27 + 112LL);
                sub_142757420(v28);
                sub_146ECA0C0(*(_QWORD *)(a1 + 16LL * *(int *)(a1 + 6728) + 6504));
                v29(v27);
                v30 = *(_QWORD *)(a1 + 6568);
                v31 = *(__int64 (__fastcall **)(__int64))(*(_QWORD *)v30 + 112LL);
                sub_142757420(*(_QWORD *)(a1 + 16LL * *(int *)(a1 + 6728) + 6504));
                sub_146ECA0C0(*(_QWORD *)(a1 + 16LL * *(int *)(a1 + 6728) + 6504));
                result = v31(v30);
              }
            }
          }
        }
        v32 = v40;
        if ( v40 )
        {
          result = (unsigned int)_InterlockedExchangeAdd(v40 + 2, 0xFFFFFFFF);
          if ( (_DWORD)result == 1 )
          {
            result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v32)(v32);
            if ( _InterlockedExchangeAdd(v32 + 3, 0xFFFFFFFF) == 1 )
              result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v32 + 8LL))(v32);
          }
        }
      }
    }
  }
  v33 = *(_QWORD *)(a1 + 6600);
  if ( v33 )
  {
    result = sub_146AEF9C0(v33);
    if ( (_BYTE)result )
    {
      result = sub_141FB6530(*(_QWORD *)(a1 + 6600));
      if ( (_BYTE)result )
      {
        if ( *(_QWORD *)(a1 + 6616) )
        {
          result = (*(__int64 (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 6600) + 488LL))(*(_QWORD *)(a1 + 6600));
          if ( (int)result >= 7 )
          {
            LOBYTE(v34) = 1;
            result = (*(__int64 (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 6616) + 16LL))(
                       *(_QWORD *)(a1 + 6616),
                       v34);
          }
        }
      }
    }
  }
  v35 = *(_QWORD *)(a1 + 6632);
  if ( v35 )
  {
    result = sub_146AEF9C0(v35);
    if ( (_BYTE)result )
    {
      result = sub_141FB6530(*(_QWORD *)(a1 + 6632));
      if ( (_BYTE)result )
      {
        if ( *(_QWORD *)(a1 + 6648) )
        {
          if ( *(_QWORD *)(a1 + 6664) )
          {
            result = (*(__int64 (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 6632) + 488LL))(*(_QWORD *)(a1 + 6632));
            if ( (int)result >= 7 )
            {
              LOBYTE(v36) = 1;
              (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 6648) + 16LL))(*(_QWORD *)(a1 + 6648), v36);
              LOBYTE(v37) = 1;
              result = (*(__int64 (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 6664) + 16LL))(
                         *(_QWORD *)(a1 + 6664),
                         v37);
            }
          }
        }
      }
    }
  }
  v38 = *(_QWORD *)(a1 + 6680);
  if ( v38 )
  {
    result = sub_146AEF960(v38);
    if ( (_BYTE)result )
    {
      result = sub_141FB6530(*(_QWORD *)(a1 + 6680));
      if ( (_BYTE)result )
        return (*(__int64 (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 6680) + 16LL))(*(_QWORD *)(a1 + 6680), 0);
    }
  }
  return result;
}

