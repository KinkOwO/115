// writer_0x1444f1be0

__int64 __fastcall sub_1444F1BE0(__int64 a1, _QWORD *a2)
{
  _QWORD *v2; // rbx
  __int64 result; // rax
  _DWORD *v4; // rdx

  v2 = (_QWORD *)(a1 + 1176);
  *(_QWORD *)(a1 + 1184) = *(_QWORD *)(a1 + 1176);
  result = sub_143D855D0(a1 + 1176, *a2, a2[1]);
  v4 = (_DWORD *)v2[1];
  if ( (_DWORD *)*v2 == v4 )
  {
    if ( v4 == (_DWORD *)v2[2] )
    {
      return sub_140154010(v2, v4, &unk_14A3176C8);
    }
    else
    {
      *v4 = 100000;
      v2[1] += 4LL;
    }
  }
  return result;
}

