// caller_1551_sub_14404C060

__int64 __fastcall sub_14404C060(__int64 a1)
{
  __int64 v2; // rax
  __int64 v3; // rax
  __int64 v4; // rax
  __int64 result; // rax
  _QWORD *v6; // rbx
  __int64 v7; // rdi
  __int64 v8; // rdx

  v2 = sub_146E8C7D0(&unk_14A1E5E70);
  if ( sub_145A0F110(v2, 226) )
  {
    v3 = sub_146E8C7D0(&unk_14A1E5E70);
    v4 = sub_145A0F110(v3, 227);
    (*(void (__fastcall **)(__int64, _QWORD, _QWORD, __int64))(*(_QWORD *)v4 + 136LL))(v4, 0, 0, 1);
  }
  sub_14404B5A0(a1);
  result = sub_14404C130(a1);
  if ( *(_DWORD *)(a1 + 2460) == 1 )
  {
    v6 = (_QWORD *)(a1 + 1624);
    v7 = 4;
    do
    {
      if ( !(unsigned __int8)sub_146AEF960(*(v6 - 10)) || (unsigned __int8)sub_141FB6530(*(v6 - 8)) )
        v8 = 0;
      else
        LOBYTE(v8) = 1;
      result = (*(__int64 (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*v6 + 16LL))(*v6, v8);
      v6 += 29;
      --v7;
    }
    while ( v7 );
  }
  return result;
}

