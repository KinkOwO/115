// writer_0x1444e9a40

__int64 __fastcall sub_1444E9A40(__int64 a1, int a2)
{
  _QWORD *v3; // rbx
  _DWORD *v4; // rcx
  __int64 result; // rax
  _DWORD *v7; // rdx
  _DWORD *v8; // rdx

  v3 = (_QWORD *)(a1 + 1128);
  v4 = *(_DWORD **)(a1 + 1128);
  result = *(_QWORD *)(a1 + 1136);
  if ( v4 != (_DWORD *)result )
  {
    while ( 1 )
    {
      v7 = v4 + 1;
      if ( *v4 == a2 )
        break;
      ++v4;
      if ( v7 == (_DWORD *)result )
        goto LABEL_6;
    }
    result = sub_148AA1E60(v4, v7, result - (_QWORD)v7);
    *(_QWORD *)(a1 + 1136) -= 4LL;
  }
LABEL_6:
  v8 = (_DWORD *)v3[1];
  if ( (_DWORD *)*v3 == v8 )
  {
    if ( v8 == (_DWORD *)v3[2] )
    {
      return sub_140154010(v3, v8, &unk_14A3176C4);
    }
    else
    {
      *v8 = 30000;
      v3[1] += 4LL;
    }
  }
  return result;
}

