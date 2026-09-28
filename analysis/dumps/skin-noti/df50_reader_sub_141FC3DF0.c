// reader_sub_141FC3DF0

unsigned __int64 __fastcall sub_141FC3DF0(__int64 a1)
{
  int v2; // ecx
  int v3; // ecx
  unsigned __int64 v4; // rbx
  __int64 v5; // rcx
  unsigned __int64 v6; // rbp
  __int64 v7; // rsi
  __int64 v8; // rcx
  __int64 v9; // rcx
  unsigned __int64 result; // rax
  __int64 v11; // rsi
  __int64 v12; // rcx
  __int64 v13; // rcx
  __int64 v14; // rsi
  __int64 v15; // rcx
  __int64 v16; // rdx
  __int64 v17; // rdx
  unsigned __int64 v18; // rbx
  __int64 v19; // rcx
  unsigned __int64 v20; // rbp
  __int64 v21; // rsi
  __int64 v22; // rcx
  unsigned __int64 v23; // rbp
  __int64 v24; // rsi
  __int64 v25; // rdx
  unsigned __int64 v26; // rbp
  __int64 v27; // rcx
  __int64 v28; // rsi
  __int64 v29; // rcx
  __int64 v30; // rdx
  __int64 v31; // rcx
  __int64 v32; // rsi
  __int64 v33; // rdx
  __int64 *j; // rbx
  __int64 v35; // rcx
  __int64 **v36; // rax
  __int64 *i; // rax
  __int64 *v38; // rcx
  __int64 v39; // rax
  int v40; // ebx

  v2 = *(_DWORD *)(a1 + 2000) - 1;
  if ( v2 )
  {
    v3 = v2 - 1;
    if ( v3 )
    {
      v4 = 0;
      if ( v3 == 1 )
      {
        v13 = *(_QWORD *)(a1 + 1952);
        result = (*(_QWORD *)(a1 + 1960) - v13) >> 4;
        if ( result )
        {
          v14 = 0;
          do
          {
            v15 = *(_QWORD *)(v14 + v13);
            if ( v15 && !(unsigned __int8)sub_141FB6530(v15) )
            {
              (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v14 + *(_QWORD *)(a1 + 1952)) + 432LL))(*(_QWORD *)(v14 + *(_QWORD *)(a1 + 1952)));
              LOBYTE(v16) = 1;
              (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v14 + *(_QWORD *)(a1 + 1952)) + 16LL))(
                *(_QWORD *)(v14 + *(_QWORD *)(a1 + 1952)),
                v16);
            }
            v13 = *(_QWORD *)(a1 + 1952);
            ++v4;
            v14 += 16;
            result = (*(_QWORD *)(a1 + 1960) - v13) >> 4;
          }
          while ( v4 < result );
        }
      }
      else
      {
        v5 = *(_QWORD *)(a1 + 1976);
        v6 = 0;
        if ( (*(_QWORD *)(a1 + 1984) - v5) >> 4 )
        {
          v7 = 0;
          do
          {
            v8 = *(_QWORD *)(v7 + v5);
            if ( v8 )
              (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v8 + 16LL))(v8, 0);
            v5 = *(_QWORD *)(a1 + 1976);
            ++v6;
            v7 += 16;
          }
          while ( v6 < (*(_QWORD *)(a1 + 1984) - v5) >> 4 );
        }
        v9 = *(_QWORD *)(a1 + 1952);
        result = (*(_QWORD *)(a1 + 1960) - v9) >> 4;
        if ( result )
        {
          v11 = 0;
          do
          {
            v12 = *(_QWORD *)(v11 + v9);
            if ( v12 )
              (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v12 + 16LL))(v12, 0);
            v9 = *(_QWORD *)(a1 + 1952);
            ++v4;
            v11 += 16;
            result = (*(_QWORD *)(a1 + 1960) - v9) >> 4;
          }
          while ( v4 < result );
        }
      }
    }
    else
    {
      v17 = *(_QWORD *)(a1 + 1984);
      v18 = 0;
      v19 = *(_QWORD *)(a1 + 1976);
      v20 = 0;
      if ( (v17 - v19) >> 4 )
      {
        v21 = 0;
        while ( 1 )
        {
          v22 = *(_QWORD *)(v21 + v19);
          if ( v22 )
          {
            if ( !(unsigned __int8)sub_141FB6530(v22)
              || (unsigned __int8)sub_141FB6530(*(_QWORD *)(v21 + *(_QWORD *)(a1 + 1976))) == 1
              && !(unsigned __int8)sub_146AEF960(*(_QWORD *)(v21 + *(_QWORD *)(a1 + 1976))) )
            {
              break;
            }
          }
          v17 = *(_QWORD *)(a1 + 1984);
          ++v20;
          v19 = *(_QWORD *)(a1 + 1976);
          v21 += 16;
          if ( v20 >= (v17 - v19) >> 4 )
            goto LABEL_29;
        }
        result = sub_141FB3FA0(*(_QWORD *)(a1 + 2024));
        if ( (_BYTE)result )
        {
          v31 = *(_QWORD *)(a1 + 1976);
          if ( (*(_QWORD *)(a1 + 1984) - v31) >> 4 )
          {
            v32 = 0;
            do
            {
              if ( !(unsigned __int8)sub_141FB6530(*(_QWORD *)(v32 + v31)) )
              {
                (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v32 + *(_QWORD *)(a1 + 1976)) + 432LL))(*(_QWORD *)(v32 + *(_QWORD *)(a1 + 1976)));
                LOBYTE(v33) = 1;
                (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v32 + *(_QWORD *)(a1 + 1976)) + 16LL))(
                  *(_QWORD *)(v32 + *(_QWORD *)(a1 + 1976)),
                  v33);
              }
              v31 = *(_QWORD *)(a1 + 1976);
              ++v18;
              v32 += 16;
            }
            while ( v18 < (*(_QWORD *)(a1 + 1984) - v31) >> 4 );
          }
          return sub_141FB42B0(*(_QWORD *)(a1 + 2024), 0);
        }
      }
      else
      {
LABEL_29:
        v23 = 0;
        if ( (v17 - v19) >> 4 )
        {
          v24 = 0;
          do
          {
            (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(v24 + *(_QWORD *)(a1 + 1976)) + 16LL))(
              *(_QWORD *)(v24 + *(_QWORD *)(a1 + 1976)),
              0);
            v24 += 16;
            ++v23;
          }
          while ( v23 < (__int64)(*(_QWORD *)(a1 + 1984) - *(_QWORD *)(a1 + 1976)) >> 4 );
        }
        v25 = *(_QWORD *)(a1 + 1960);
        v26 = 0;
        v27 = *(_QWORD *)(a1 + 1952);
        if ( (v25 - v27) >> 4 )
        {
          v28 = 0;
          do
          {
            v29 = *(_QWORD *)(v28 + v27);
            if ( v29 && !(unsigned __int8)sub_141FB6530(v29) )
            {
              (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(v28 + *(_QWORD *)(a1 + 1952)) + 432LL))(*(_QWORD *)(v28 + *(_QWORD *)(a1 + 1952)));
              LOBYTE(v30) = 1;
              (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(v28 + *(_QWORD *)(a1 + 1952)) + 16LL))(
                *(_QWORD *)(v28 + *(_QWORD *)(a1 + 1952)),
                v30);
            }
            v25 = *(_QWORD *)(a1 + 1960);
            ++v26;
            v27 = *(_QWORD *)(a1 + 1952);
            v28 += 16;
          }
          while ( v26 < (v25 - v27) >> 4 );
        }
        result = 3;
        if ( v27 == v25 )
          result = 0;
        *(_DWORD *)(a1 + 2000) = result;
      }
    }
  }
  else
  {
    j = **(__int64 ***)(a1 + 1920);
    if ( *((_BYTE *)j + 25) )
    {
LABEL_60:
      v39 = sub_1401E65D0(*(_QWORD *)(a1 + 2024));
      v40 = sub_140BB0090(v39);
      result = sub_141FB3A40(*(_QWORD *)(a1 + 2024));
      if ( (_DWORD)result == v40 )
      {
        if ( *(_QWORD *)(a1 + 1976) == *(_QWORD *)(a1 + 1984)
          || (result = sub_141FB3FA0(*(_QWORD *)(a1 + 2024)), !(_BYTE)result) )
        {
          *(_DWORD *)(a1 + 2000) = 3;
          return 3;
        }
        else
        {
          *(_DWORD *)(a1 + 2000) = 2;
        }
      }
    }
    else
    {
      while ( 1 )
      {
        v35 = j[5];
        if ( v35 )
        {
          if ( (unsigned __int8)sub_141FB6530(v35) )
          {
            result = sub_146AEF960(j[5]);
            if ( !(_BYTE)result )
              break;
          }
        }
        v36 = (__int64 **)j[2];
        if ( *((_BYTE *)v36 + 25) )
        {
          for ( i = (__int64 *)j[1]; !*((_BYTE *)i + 25); i = (__int64 *)i[1] )
          {
            if ( j != (__int64 *)i[2] )
              break;
            j = i;
          }
          j = i;
        }
        else
        {
          v38 = *v36;
          for ( j = (__int64 *)j[2]; !*((_BYTE *)v38 + 25); v38 = (__int64 *)*v38 )
            j = v38;
        }
        if ( *((_BYTE *)j + 25) )
          goto LABEL_60;
      }
    }
  }
  return result;
}

