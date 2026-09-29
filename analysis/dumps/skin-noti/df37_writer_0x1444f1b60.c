// writer_0x1444f1b60

__int64 __fastcall sub_1444F1B60(__int64 a1, _QWORD *a2)
{
  _QWORD *v3; // rbx
  _DWORD *v4; // rdx

  v3 = (_QWORD *)(a1 + 1128);
  *(_QWORD *)(a1 + 1136) = *(_QWORD *)(a1 + 1128);
  sub_143D855D0(a1 + 1128, *a2, a2[1]);
  v4 = (_DWORD *)v3[1];
  if ( (_DWORD *)*v3 == v4 )
  {
    if ( v4 == (_DWORD *)v3[2] )
    {
      sub_140154010(v3, v4, &unk_14A3176C4);
    }
    else
    {
      *v4 = 30000;
      v3[1] += 4LL;
    }
  }
  return sub_1401574A0(a2);
}

