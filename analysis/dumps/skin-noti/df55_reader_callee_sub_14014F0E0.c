// reader_callee_sub_14014F0E0

__int64 __fastcall sub_14014F0E0(_QWORD *a1, __int64 a2, __int64 a3)
{
  _QWORD *v4; // rdi
  _QWORD *v6; // rax
  _QWORD *v8; // rdx
  _QWORD *v9; // rcx
  _QWORD *v10; // r9
  __int64 *v11; // r8
  _QWORD *v12; // r10
  _QWORD *v13; // rax
  _QWORD *v14; // r8
  _QWORD *v15; // rax
  _QWORD *v16; // rcx
  _QWORD *v17; // r8
  __int64 v18; // rax
  _QWORD *v19; // rax
  _QWORD *v20; // rax
  __int64 v21; // rax
  _QWORD *v22; // rax
  _QWORD *v23; // rax

  ++a1[1];
  v4 = (_QWORD *)*a1;
  v6 = *(_QWORD **)a2;
  *(_QWORD *)(a3 + 8) = *(_QWORD *)a2;
  if ( v6 == v4 )
  {
    *v4 = a3;
    v4[1] = a3;
    v4[2] = a3;
    *(_BYTE *)(a3 + 24) = 1;
    return a3;
  }
  if ( *(_DWORD *)(a2 + 8) )
  {
    *v6 = a3;
    if ( v6 == (_QWORD *)*v4 )
      *v4 = a3;
  }
  else
  {
    v6[2] = a3;
    if ( v6 == (_QWORD *)v4[2] )
      v4[2] = a3;
  }
  v8 = (_QWORD *)a3;
  while ( !*(_BYTE *)(v8[1] + 24LL) )
  {
    v9 = (_QWORD *)v8[1];
    v10 = v8 + 1;
    v11 = (__int64 *)v9[1];
    v12 = v9 + 1;
    v13 = (_QWORD *)*v11;
    if ( v9 == (_QWORD *)*v11 )
    {
      v13 = (_QWORD *)v11[2];
      if ( *((_BYTE *)v13 + 24) )
      {
        v14 = (_QWORD *)v9[2];
        if ( v8 == v14 )
        {
          v8 = (_QWORD *)v8[1];
          v9[2] = *v14;
          if ( !*(_BYTE *)(*v14 + 25LL) )
            *(_QWORD *)(*v14 + 8LL) = v9;
          v14[1] = *v12;
          if ( v9 == *(_QWORD **)(*a1 + 8LL) )
          {
            *(_QWORD *)(*a1 + 8LL) = v14;
            v10 = v9 + 1;
            *v14 = v9;
            *v12 = v14;
          }
          else
          {
            v15 = (_QWORD *)*v12;
            if ( v9 == *(_QWORD **)*v12 )
              *v15 = v14;
            else
              v15[2] = v14;
            v10 = v9 + 1;
            *v14 = v9;
            *v12 = v14;
          }
        }
        else
        {
          v14 = (_QWORD *)v8[1];
        }
        *((_BYTE *)v14 + 24) = 1;
        *(_BYTE *)(*(_QWORD *)(*v10 + 8LL) + 24LL) = 0;
        v16 = *(_QWORD **)(*v10 + 8LL);
        v17 = (_QWORD *)*v16;
        *v16 = *(_QWORD *)(*v16 + 16LL);
        v18 = v17[2];
        if ( !*(_BYTE *)(v18 + 25) )
          *(_QWORD *)(v18 + 8) = v16;
        v17[1] = v16[1];
        if ( v16 == *(_QWORD **)(*a1 + 8LL) )
        {
          *(_QWORD *)(*a1 + 8LL) = v17;
          v17[2] = v16;
        }
        else
        {
          v19 = (_QWORD *)v16[1];
          if ( v16 == (_QWORD *)v19[2] )
            v19[2] = v17;
          else
            *v19 = v17;
          v17[2] = v16;
        }
LABEL_48:
        v16[1] = v17;
        continue;
      }
    }
    else if ( *((_BYTE *)v13 + 24) )
    {
      v20 = (_QWORD *)*v9;
      if ( v8 == (_QWORD *)*v9 )
      {
        v8 = (_QWORD *)v8[1];
        v9 = (_QWORD *)*v9;
        *v8 = v20[2];
        v21 = v20[2];
        if ( !*(_BYTE *)(v21 + 25) )
          *(_QWORD *)(v21 + 8) = v8;
        v9[1] = *v12;
        if ( v8 == *(_QWORD **)(*a1 + 8LL) )
        {
          *(_QWORD *)(*a1 + 8LL) = v9;
        }
        else
        {
          v22 = (_QWORD *)*v12;
          if ( v8 == *(_QWORD **)(*v12 + 16LL) )
            v22[2] = v9;
          else
            *v22 = v9;
        }
        v9[2] = v8;
        v10 = v12;
        *v12 = v9;
      }
      *((_BYTE *)v9 + 24) = 1;
      *(_BYTE *)(*(_QWORD *)(*v10 + 8LL) + 24LL) = 0;
      v16 = *(_QWORD **)(*v10 + 8LL);
      v17 = (_QWORD *)v16[2];
      v16[2] = *v17;
      if ( !*(_BYTE *)(*v17 + 25LL) )
        *(_QWORD *)(*v17 + 8LL) = v16;
      v17[1] = v16[1];
      if ( v16 == *(_QWORD **)(*a1 + 8LL) )
      {
        *(_QWORD *)(*a1 + 8LL) = v17;
      }
      else
      {
        v23 = (_QWORD *)v16[1];
        if ( v16 == (_QWORD *)*v23 )
          *v23 = v17;
        else
          v23[2] = v17;
      }
      *v17 = v16;
      goto LABEL_48;
    }
    *((_BYTE *)v9 + 24) = 1;
    *((_BYTE *)v13 + 24) = 1;
    *(_BYTE *)(*(_QWORD *)(*v10 + 8LL) + 24LL) = 0;
    v8 = *(_QWORD **)(*v10 + 8LL);
  }
  *(_BYTE *)(v4[1] + 24LL) = 1;
  return a3;
}

