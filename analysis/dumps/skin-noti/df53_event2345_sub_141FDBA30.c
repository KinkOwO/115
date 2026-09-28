// event2345_sub_141FDBA30

__int64 __fastcall sub_141FDBA30(_QWORD *a1, unsigned int a2, unsigned int a3, __int64 a4)
{
  _QWORD *v8; // rbx
  __int64 v9; // rax
  __int64 v10; // rax
  __int64 v11; // rbx
  int v12; // eax
  __int64 v13; // rdx
  volatile signed __int32 *v14; // rbx
  __int64 v16; // [rsp+38h] [rbp-30h] BYREF
  volatile signed __int32 *v17; // [rsp+40h] [rbp-28h]

  if ( a3 == 13 )
  {
    v8 = a1 - 167;
    sub_145F6E370(a1 - 167, &v16, a2);
    v9 = v16;
    if ( v16 == a1[22] )
    {
      (*(void (__fastcall **)(_QWORD *))(*v8 + 264LL))(v8);
      v9 = v16;
    }
    if ( v9 == a1[24] )
    {
      (*(void (__fastcall **)(_QWORD *))(*v8 + 264LL))(v8);
      sub_146694510(qword_14E683C78, 2345, -1, 0, 1);
      v9 = v16;
    }
    if ( v9 == a1[26] )
    {
      (*(void (__fastcall **)(_QWORD *))(*v8 + 264LL))(v8);
      v10 = sub_14667BB90(qword_14E683C78, 2345, 0);
      v11 = v10;
      if ( v10 )
      {
        sub_1421B2820(*(_QWORD *)(v10 + 4288));
        LOBYTE(v13) = v12 == 0;
        sub_141FDD670(v11, v13);
      }
    }
    v14 = v17;
    if ( v17 )
    {
      if ( _InterlockedExchangeAdd(v17 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v14)(v14);
        if ( _InterlockedExchangeAdd(v14 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v14 + 8LL))(v14);
      }
    }
  }
  return sub_14018E630(a1, a2, a3, a4);
}

