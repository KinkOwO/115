// sub_1444EB0A0

_QWORD *__fastcall sub_1444EB0A0(__int64 a1, _QWORD *a2, __int64 a3)
{
  unsigned int v3; // eax
  __int64 v6; // rax
  __int64 **v7; // r14
  __int64 *v8; // rcx
  __int64 *v9; // rdx
  __int64 v10; // r8
  __int64 v11; // rax
  _QWORD *v12; // rdx
  __int64 v13; // rbx
  __int64 v14; // rax
  volatile signed __int32 *v15; // rbx
  __int64 v16; // rbx
  _QWORD *v17; // rax
  int v18; // eax
  _QWORD *v19; // rax
  __int64 v20; // rcx
  _BYTE v22[8]; // [rsp+30h] [rbp-28h] BYREF
  volatile signed __int32 *v23; // [rsp+38h] [rbp-20h]
  signed int v24; // [rsp+70h] [rbp+18h] BYREF

  v24 = a3;
  v3 = a3;
  LOBYTE(a3) = 1;
  v6 = sub_140283D60(qword_14E683BF8, v3, a3);
  if ( !v6 || *(_DWORD *)(v6 + 8) )
  {
    *a2 = 0;
    a2[1] = 0;
  }
  else
  {
    v7 = (__int64 **)(a1 + 1032);
    v8 = (__int64 *)(*v7)[1];
    v9 = *v7;
    v10 = (unsigned int)v24;
    while ( !*((_BYTE *)v8 + 25) )
    {
      if ( *((_DWORD *)v8 + 8) >= v24 )
      {
        v9 = v8;
        v8 = (__int64 *)*v8;
      }
      else
      {
        v8 = (__int64 *)v8[2];
      }
    }
    if ( *((_BYTE *)v9 + 25) || v24 < *((_DWORD *)v9 + 8) || v9 == *v7 || !v9[5] )
    {
      v12 = *(_QWORD **)(v6 + 680);
      if ( v12 == *(_QWORD **)(v6 + 688) )
      {
        *a2 = 0;
        a2[1] = 0;
      }
      else
      {
        if ( v12[3] >= 8u )
          v12 = (_QWORD *)*v12;
        LOBYTE(v10) = 1;
        v13 = sub_144724390(v22, v12, v10, 0);
        v14 = sub_140457BE0(v7, &v24);
        sub_1401E5080(v14, v13);
        v15 = v23;
        if ( v23 )
        {
          if ( _InterlockedExchangeAdd(v23 + 2, 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v15)(v15);
            if ( _InterlockedExchangeAdd(v15 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v15 + 8LL))(v15);
          }
        }
        v16 = *(_QWORD *)sub_140457BE0(v7, &v24);
        v17 = (_QWORD *)sub_140457BE0(v7, &v24);
        v18 = sub_146B33E50(*v17);
        sub_146B5E370(v16, (unsigned int)(v18 - 1), 0);
        v19 = (_QWORD *)sub_140457BE0(v7, &v24);
        *a2 = 0;
        a2[1] = 0;
        v20 = v19[1];
        if ( v20 )
          _InterlockedIncrement((volatile signed __int32 *)(v20 + 8));
        *a2 = *v19;
        a2[1] = v19[1];
      }
    }
    else
    {
      *a2 = 0;
      a2[1] = 0;
      v11 = v9[6];
      if ( v11 )
        _InterlockedIncrement((volatile signed __int32 *)(v11 + 8));
      *a2 = v9[5];
      a2[1] = v9[6];
    }
  }
  return a2;
}

