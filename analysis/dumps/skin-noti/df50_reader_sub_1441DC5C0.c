// reader_sub_1441DC5C0

__int64 __fastcall sub_1441DC5C0(_QWORD *a1)
{
  __int64 v2; // rcx
  __int64 v3; // rdi
  __int64 (__fastcall *v4)(__int64, __int64); // rsi
  __int64 v5; // rdx

  if ( !(unsigned __int8)sub_146ECA4A0(a1[36])
    || !(unsigned __int8)sub_146ED0010(a1[6])
    && !(unsigned __int8)sub_146ED0010(a1[14])
    && !(unsigned __int8)sub_146ED0010(a1[34])
    && !(unsigned __int8)sub_146ED0010(a1[28])
    && !(unsigned __int8)sub_146ED0010(a1[30]) )
  {
    v2 = a1[1];
    if ( !v2 || !(unsigned __int8)sub_146ED0010(v2) )
    {
      v3 = a1[36];
      v4 = *(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v3 + 16LL);
      goto LABEL_12;
    }
  }
  v3 = a1[36];
  v4 = *(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v3 + 16LL);
  if ( (unsigned __int8)sub_141FB6530(a1[38]) )
  {
LABEL_12:
    v5 = 0;
    return v4(v3, v5);
  }
  LOBYTE(v5) = 1;
  return v4(v3, v5);
}

