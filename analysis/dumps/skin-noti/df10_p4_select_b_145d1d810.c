__int64 __fastcall sub_145D1D810(__int64 a1, __int64 a2)
{
  __int64 v3; // rdi
  __int64 v4; // rax
  __int64 v5; // rax
  _BYTE v7[32]; // [rsp+38h] [rbp-20h] BYREF

  v3 = *(_QWORD *)(a1 + 24648);
  LOBYTE(a2) = 1;
  sub_140B87FB0(qword_14E684B10, a2);
  sub_145C432F0(a1);
  v4 = *(_QWORD *)(a1 + 24648);
  if ( v4 != v3 )
  {
    if ( v3 )
    {
      sub_14033D360(v7, v3);
      sub_145D21E70(a1, v7, 12);
      sub_14033E820(v7);
      v4 = *(_QWORD *)(a1 + 24648);
    }
    if ( v4 )
    {
      v5 = sub_145C20190(a1);
      sub_14033D360(v7, v5);
      sub_145C994B0(a1, (unsigned int)v7, 12, 3, 0);
      sub_14033E820(v7);
    }
  }
  return sub_140B87FB0(qword_14E684B10, 0);
}
