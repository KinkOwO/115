// global_writer_sub_146ED14B0

__int64 __fastcall sub_146ED14B0(__int64 a1, _QWORD *a2)
{
  unsigned int v4; // ebx
  __int64 v5; // rcx
  volatile signed __int32 *v6; // rbx
  __int64 v7; // rcx
  unsigned int v8; // esi
  volatile signed __int32 *v9; // rbx
  __int64 v11; // [rsp+38h] [rbp-30h] BYREF
  volatile signed __int32 *v12; // [rsp+40h] [rbp-28h]
  unsigned __int64 v13; // [rsp+70h] [rbp+8h]

  byte_14F1C1FA6 = 1;
  dword_14DC6B680 = -1;
  sub_146EC5120(0);
  dword_14DC6B674 = *(_DWORD *)(a1 + 496);
  if ( *a2 )
  {
    v4 = *(_DWORD *)(*a2 + 496LL);
    if ( (unsigned __int8)sub_146F54360(v4) )
    {
      dword_14DC6B63C = v4;
      dword_14DC6B66C = v4;
      sub_146ED4690(a1, &v11, v4);
      if ( v11 )
      {
        v5 = *(_QWORD *)(v11 + 552);
        if ( v5 )
          (*(void (__fastcall **)(__int64, __int64 *, __int64, _QWORD, _QWORD))(*(_QWORD *)v5 + 8LL))(
            v5,
            &v11,
            13,
            0,
            0);
      }
      v6 = v12;
      if ( v12 )
      {
        if ( _InterlockedExchangeAdd(v12 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v6)(v6);
          if ( _InterlockedExchangeAdd(v6 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v6 + 8LL))(v6);
        }
      }
    }
    dword_14DC6B670 = *(_DWORD *)(*a2 + 496LL);
  }
  if ( *(_DWORD *)(a1 + 136) == 2 )
  {
    dword_14DC6B680 = *(_DWORD *)(a1 + 496);
  }
  else
  {
    if ( *(_DWORD *)(a1 + 136) != 8 )
    {
      if ( *(_DWORD *)(a1 + 136) == 9 )
      {
        sub_146ED3470(a1, *(unsigned int *)(a1 + 496));
        goto LABEL_21;
      }
      if ( *(_DWORD *)(a1 + 136) == 13 )
      {
        dword_14DC6B684 = *(_DWORD *)(a1 + 496);
        goto LABEL_21;
      }
      if ( *(_DWORD *)(a1 + 136) != 18 )
        goto LABEL_21;
    }
    dword_14DC6B67C = *(_DWORD *)(a1 + 496);
    if ( *(_DWORD *)(*a2 + 136LL) == 7 )
      dword_14DC6B688 = *(_DWORD *)(*a2 + 496LL);
  }
LABEL_21:
  if ( *(_DWORD *)(*a2 + 136LL) == 9 )
    sub_146ED3470(*a2, *(unsigned int *)(*a2 + 496LL));
  dword_14DC6B650 = -1;
  if ( *a2 )
  {
    dword_14DC6B654 = *(_DWORD *)(*a2 + 496LL);
    v7 = *(_QWORD *)(*a2 + 552LL);
    if ( v7 )
      (*(void (__fastcall **)(__int64, _QWORD *, __int64))(*(_QWORD *)v7 + 8LL))(v7, a2, 10);
    dword_14DC6B648 = *(_DWORD *)(*a2 + 496LL);
  }
  if ( qword_14F0EA870 )
  {
    v13 = sub_146EC45D0(*(_QWORD *)(a1 + 400));
    sub_146D889F0(qword_14F0EA870, (unsigned int)v13, HIDWORD(v13));
    sub_146D88B40(qword_14F0EA870, (unsigned int)v13, HIDWORD(v13));
  }
  v8 = dword_14DC6B670;
  v9 = (volatile signed __int32 *)a2[1];
  if ( v9 )
  {
    if ( _InterlockedExchangeAdd(v9 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v9)(v9);
      if ( _InterlockedExchangeAdd(v9 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v9 + 8LL))(v9);
    }
  }
  return v8;
}

