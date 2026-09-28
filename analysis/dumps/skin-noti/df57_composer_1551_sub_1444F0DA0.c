// composer_1551_sub_1444F0DA0

unsigned __int64 __fastcall sub_1444F0DA0(__int64 a1, __int64 a2)
{
  unsigned __int64 v3; // rdi
  __int64 v4; // rsi
  __int64 v5; // r8
  __int64 v6; // rcx
  unsigned __int64 result; // rax
  __int64 i; // rbx
  unsigned __int16 *v9; // rax
  __int64 v10; // r8
  __int64 v11; // rcx
  int v12; // edx
  __int64 v13; // rcx
  __int64 v14; // rax
  __int64 v15; // rcx
  __int64 v16; // rax
  __int64 v17; // rdx
  __int64 v18; // rcx

  v3 = 0;
  v4 = a1 + 24LL * (int)sub_1444EAA90();
  v5 = *(_QWORD *)(v4 + 1360);
  v6 = *(_QWORD *)(v4 + 1368) - v5;
  result = (unsigned __int64)(v6 + ((unsigned __int128)(v6 * (__int128)(__int64)0xA3D70A3D70A3D70BuLL) >> 64)) >> 63;
  if ( v6 / 25 )
  {
    for ( i = 0; ; i += 25 )
    {
      if ( *(_DWORD *)(i + v5) )
      {
        v9 = (unsigned __int16 *)sub_146E90B00(i + v5 + 4);
        v10 = a2 - (_QWORD)v9;
        do
        {
          v11 = *(unsigned __int16 *)((char *)v9 + v10);
          v12 = *v9 - (_DWORD)v11;
          if ( v12 )
            break;
          ++v9;
        }
        while ( (_DWORD)v11 );
        if ( !v12 )
          break;
      }
      v5 = *(_QWORD *)(v4 + 1360);
      ++v3;
      v13 = *(_QWORD *)(v4 + 1368) - v5;
      result = (unsigned __int64)(v13 + ((unsigned __int128)(v13 * (__int128)(__int64)0xA3D70A3D70A3D70BuLL) >> 64)) >> 63;
      if ( v3 >= v13 / 25 )
        return result;
    }
    v14 = sub_146D74000(v11);
    sub_146D746E0(v14, 1551);
    v16 = sub_146D74000(v15);
    sub_146D75CE0(v16, *(unsigned int *)(25 * v3 + *(_QWORD *)(v4 + 1360)));
    return sub_146D75AF0(v18, v17);
  }
  return result;
}

